"""
Document processor for chunking and cleaning text.

Implements intelligent text chunking strategies for RAG systems.
"""

import hashlib
import logging
import re
from dataclasses import dataclass, field
from typing import Optional

import tiktoken

logger = logging.getLogger(__name__)

# Default encoding for token counting
ENCODING = "cl100k_base"  # GPT-4 / Claude compatible


@dataclass
class Document:
    """A document to be processed and indexed."""

    id: str
    title: str
    content: str
    source: Optional[str] = None
    metadata: dict = field(default_factory=dict)

    def __post_init__(self):
        if self.metadata is None:
            self.metadata = {}


@dataclass
class Chunk:
    """A chunk of a document for embedding."""

    id: str
    document_id: str
    content: str
    token_count: int
    chunk_index: int
    embedding: Optional[list[float]] = None
    metadata: dict = field(default_factory=dict)

    def __post_init__(self):
        if self.metadata is None:
            self.metadata = {}


@dataclass
class ChunkingConfig:
    """Configuration for text chunking."""

    chunk_size: int = 512  # Target tokens per chunk
    chunk_overlap: int = 50  # Overlap between chunks
    min_chunk_size: int = 100  # Minimum tokens for a valid chunk
    max_chunk_size: int = 1024  # Maximum tokens per chunk


class DocumentProcessor:
    """Processes documents into chunks suitable for embedding."""

    def __init__(self, config: Optional[ChunkingConfig] = None):
        self.config = config or ChunkingConfig()
        self._encoding = tiktoken.get_encoding(ENCODING)

    def process_document(self, document: Document) -> list[Chunk]:
        """
        Process a document into chunks.

        Args:
            document: The document to process

        Returns:
            List of chunks with metadata
        """
        # Clean and preprocess content
        content = self._clean_text(document.content)

        # Split into sections (by headers, paragraphs, etc.)
        sections = self._split_into_sections(content)

        # Create chunks from sections
        chunks = []
        chunk_index = 0

        for section in sections:
            section_chunks = self._chunk_section(
                section=section,
                document=document,
                start_index=chunk_index,
            )
            chunks.extend(section_chunks)
            chunk_index += len(section_chunks)

        logger.info(
            f"Processed document '{document.title}' into {len(chunks)} chunks"
        )
        return chunks

    def process_documents(self, documents: list[Document]) -> list[Chunk]:
        """Process multiple documents into chunks."""
        all_chunks = []
        for doc in documents:
            chunks = self.process_document(doc)
            all_chunks.extend(chunks)
        return all_chunks

    def _clean_text(self, text: str) -> str:
        """Clean and normalize text."""
        # Remove excessive whitespace
        text = re.sub(r"\s+", " ", text)

        # Remove excessive newlines
        text = re.sub(r"\n{3,}", "\n\n", text)

        # Remove common web artifacts
        text = re.sub(r"(?i)(cookie|subscribe|newsletter|sign up|log in).*", "", text)

        # Normalize quotes and dashes (using Unicode code points)
        text = text.replace('\u201c', '"').replace('\u201d', '"')  # Left/right double quotes
        text = text.replace('\u2018', "'").replace('\u2019', "'")  # Left/right single quotes
        text = text.replace('\u2014', '-').replace('\u2013', '-')  # Em dash, en dash

        return text.strip()

    def _split_into_sections(self, text: str) -> list[str]:
        """
        Split text into logical sections.

        Respects document structure (headers, paragraphs).
        """
        # Split by double newlines (paragraphs)
        paragraphs = text.split("\n\n")

        sections = []
        current_section = []

        for para in paragraphs:
            para = para.strip()
            if not para:
                continue

            # Check if this is a header (short, might end with : or be all caps)
            is_header = (
                len(para) < 100
                and (para.endswith(":") or para.isupper() or para.startswith("#"))
            )

            if is_header and current_section:
                # Save current section and start new one
                sections.append("\n\n".join(current_section))
                current_section = [para]
            else:
                current_section.append(para)

        # Add last section
        if current_section:
            sections.append("\n\n".join(current_section))

        return sections

    def _chunk_section(
        self,
        section: str,
        document: Document,
        start_index: int,
    ) -> list[Chunk]:
        """Chunk a section into appropriately sized pieces."""
        tokens = self._encoding.encode(section)

        # If section is small enough, return as single chunk
        if len(tokens) <= self.config.max_chunk_size:
            return [self._create_chunk(
                content=section,
                document=document,
                chunk_index=start_index,
            )]

        # Otherwise, split into multiple chunks with overlap
        chunks = []
        chunk_index = start_index
        start = 0

        while start < len(tokens):
            # Get chunk tokens
            end = min(start + self.config.chunk_size, len(tokens))
            chunk_tokens = tokens[start:end]

            # Decode back to text
            chunk_text = self._encoding.decode(chunk_tokens)

            # Only create chunk if it meets minimum size
            if len(chunk_tokens) >= self.config.min_chunk_size:
                chunks.append(self._create_chunk(
                    content=chunk_text,
                    document=document,
                    chunk_index=chunk_index,
                ))
                chunk_index += 1

            # Move start position with overlap
            start = end - self.config.chunk_overlap
            if start <= chunks[-1].token_count if chunks else 0:
                break  # Prevent infinite loop

        return chunks

    def _create_chunk(
        self,
        content: str,
        document: Document,
        chunk_index: int,
    ) -> Chunk:
        """Create a chunk with proper metadata."""
        token_count = len(self._encoding.encode(content))

        # Generate unique chunk ID
        chunk_id = self._generate_chunk_id(document.id, chunk_index, content)

        return Chunk(
            id=chunk_id,
            document_id=document.id,
            content=content,
            token_count=token_count,
            chunk_index=chunk_index,
            metadata={
                "document_title": document.title,
                "document_source": document.source,
                **document.metadata,
            },
        )

    def _generate_chunk_id(
        self,
        document_id: str,
        chunk_index: int,
        content: str,
    ) -> str:
        """Generate a unique ID for a chunk."""
        hash_input = f"{document_id}_{chunk_index}_{content[:50]}"
        return hashlib.md5(hash_input.encode()).hexdigest()[:16]

    def count_tokens(self, text: str) -> int:
        """Count tokens in text."""
        return len(self._encoding.encode(text))


__all__ = [
    "Document",
    "Chunk",
    "ChunkingConfig",
    "DocumentProcessor",
]