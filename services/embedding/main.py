#!/usr/bin/env python3
"""
UTA Embedding Service - gRPC Server Entry Point
"""

import asyncio
import logging
import os
import sys

# Set offline mode BEFORE any imports
os.environ['HF_HUB_OFFLINE'] = '1'
os.environ['TRANSFORMERS_OFFLINE'] = '1'

# Add current directory to path for imports
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from src.grpc_service import serve

if __name__ == "__main__":
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )
    logger = logging.getLogger(__name__)
    logger.info("Starting UTA Embedding gRPC Service...")
    logger.info(f"PYTHONPATH: {sys.path}")

    try:
        asyncio.run(serve())
    except KeyboardInterrupt:
        logger.info("Service stopped by user")
    except Exception as e:
        logger.error(f"Service error: {e}", exc_info=True)
        sys.exit(1)
