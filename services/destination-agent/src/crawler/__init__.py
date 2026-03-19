"""
Web crawler for gathering destination information.
"""

import asyncio
import logging
from dataclasses import dataclass
from typing import Optional
from urllib.parse import urljoin, urlparse

import httpx
from bs4 import BeautifulSoup

logger = logging.getLogger(__name__)


@dataclass
class CrawlResult:
    """Result of crawling a web page."""

    url: str
    title: str
    content: str
    links: list[str]
    metadata: dict


class WebCrawler:
    """Crawls travel websites for destination information."""

    def __init__(self, max_concurrent: int = 5, timeout: int = 30):
        self.max_concurrent = max_concurrent
        self.timeout = timeout
        self._semaphore = asyncio.Semaphore(max_concurrent)
        self._visited: set[str] = set()

    async def crawl(
        self,
        start_urls: list[str],
        max_pages: int = 50,
        allowed_domains: Optional[list[str]] = None,
    ) -> list[CrawlResult]:
        """
        Crawl URLs and extract content.

        Args:
            start_urls: Initial URLs to crawl
            max_pages: Maximum number of pages to crawl
            allowed_domains: Only crawl these domains (None = all)
        """
        results: list[CrawlResult] = []
        queue = list(start_urls)

        async with httpx.AsyncClient(timeout=self.timeout) as client:
            while queue and len(results) < max_pages:
                url = queue.pop(0)

                if url in self._visited:
                    continue

                if allowed_domains and urlparse(url).netloc not in allowed_domains:
                    continue

                async with self._semaphore:
                    result = await self._crawl_page(client, url)

                if result:
                    results.append(result)
                    self._visited.add(url)

                    # Add discovered links to queue
                    for link in result.links:
                        if link not in self._visited and link not in queue:
                            queue.append(link)

        return results

    async def _crawl_page(
        self,
        client: httpx.AsyncClient,
        url: str,
    ) -> Optional[CrawlResult]:
        """Crawl a single page."""
        try:
            logger.info(f"Crawling: {url}")
            response = await client.get(url, follow_redirects=True)
            response.raise_for_status()

            soup = BeautifulSoup(response.text, "html.parser")

            # Extract title
            title = soup.title.string.strip() if soup.title else ""

            # Extract main content (simplified)
            # TODO: Use trafilatura for better extraction
            content = self._extract_content(soup)

            # Extract links
            links = []
            for a in soup.find_all("a", href=True):
                link = urljoin(url, a["href"])
                if link.startswith(("http://", "https://")):
                    links.append(link)

            return CrawlResult(
                url=url,
                title=title,
                content=content,
                links=links,
                metadata={"status_code": response.status_code},
            )

        except Exception as e:
            logger.error(f"Failed to crawl {url}: {e}")
            return None

    def _extract_content(self, soup: BeautifulSoup) -> str:
        """Extract main content from a page."""
        # Remove scripts, styles, navigation
        for element in soup.find_all(["script", "style", "nav", "header", "footer"]):
            element.decompose()

        # Get text content
        text = soup.get_text(separator="\n", strip=True)

        # Clean up whitespace
        lines = [line.strip() for line in text.split("\n") if line.strip()]
        return "\n".join(lines)

    def reset(self) -> None:
        """Reset the crawler state."""
        self._visited.clear()


__all__ = ["WebCrawler", "CrawlResult"]