#!/usr/bin/env python3
"""
Tavily Real-time Search Script

Requires: TAVILY_API_KEY environment variable

Usage:
    python tavily_search.py "query"

Examples:
    python tavily_search.py "京都 门票价格 2024"
    python tavily_search.py "Tokyo weather tomorrow"
"""

import json
import os
import sys
import urllib.request


def search_tavily(query: str, max_results: int = 5) -> dict:
    """
    Search using Tavily API for real-time results.

    Args:
        query: Search query
        max_results: Max results to return

    Returns:
        Dict with 'success' and 'results' or 'error'
    """
    api_key = os.environ.get("TAVILY_API_KEY")
    if not api_key:
        return {"success": False, "error": "TAVILY_API_KEY not set"}

    url = "https://api.tavily.com/search"
    data = {
        "api_key": api_key,
        "query": query,
        "search_depth": "basic",
        "max_results": max_results,
    }

    try:
        # Setup proxy if available
        http_proxy = os.environ.get("HTTP_PROXY") or os.environ.get("http_proxy")
        https_proxy = os.environ.get("HTTPS_PROXY") or os.environ.get("https_proxy")

        req = urllib.request.Request(
            url,
            data=json.dumps(data).encode(),
            headers={"Content-Type": "application/json"},
        )

        if https_proxy:
            req.set_proxy(https_proxy, "https")
        elif http_proxy:
            req.set_proxy(http_proxy, "http")

        with urllib.request.urlopen(req, timeout=30) as response:
            result = json.loads(response.read().decode())

        results = []
        for item in result.get("results", []):
            results.append({
                "title": item.get("title", ""),
                "url": item.get("url", ""),
                "content": item.get("content", ""),
                "score": item.get("score", 0),
            })

        return {"success": True, "results": results}

    except Exception as e:
        return {"success": False, "error": str(e)}


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    query = sys.argv[1]
    result = search_tavily(query)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
