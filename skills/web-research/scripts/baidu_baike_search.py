#!/usr/bin/env python3
"""
Baidu Baike Search Script - Search Chinese encyclopedia

Usage:
    python baidu_baike_search.py "query"

Examples:
    python baidu_baike_search.py "故宫博物院"
    python baidu_baike_search.py "苏州园林"

Environment:
    HTTP_PROXY / HTTPS_PROXY - Optional proxy settings
"""

import json
import os
import re
import sys
import urllib.request
import urllib.parse


def search_baidu_baike(query: str, timeout: int = 15) -> dict:
    """
    Search Baidu Baike (百度百科) for Chinese content.

    Args:
        query: Search query (Chinese)
        timeout: Request timeout in seconds

    Returns:
        Dict with 'success' and 'results' or 'error'
    """
    # Setup proxy if available
    http_proxy = os.environ.get("HTTP_PROXY") or os.environ.get("http_proxy")
    https_proxy = os.environ.get("HTTPS_PROXY") or os.environ.get("https_proxy")

    try:
        # Search URL
        encoded_query = urllib.parse.quote(query)
        search_url = f"https://baike.baidu.com/search?word={encoded_query}&pn=0&rn=5"

        req = urllib.request.Request(
            search_url,
            headers={
                "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
                "Accept": "text/html,application/xhtml+xml",
                "Accept-Language": "zh-CN,zh;q=0.9",
            }
        )

        # Handle proxy
        if https_proxy:
            req.set_proxy(https_proxy, "https")
        elif http_proxy:
            req.set_proxy(http_proxy, "http")

        with urllib.request.urlopen(req, timeout=timeout) as response:
            html = response.read().decode("utf-8", errors="replace")

        # Parse search results
        results = parse_baike_results(html, query)

        return {
            "success": True,
            "query": query,
            "source": "百度百科",
            "results": results
        }

    except Exception as e:
        return {
            "success": False,
            "error": str(e),
            "query": query
        }


def parse_baike_results(html: str, query: str) -> list:
    """Parse Baidu Baike search results from HTML."""
    results = []

    # Extract search result items
    # Pattern: <a class="result-title" href="URL">Title</a>
    title_pattern = r'<a[^>]*class="[^"]*result-title[^"]*"[^>]*href="([^"]+)"[^>]*>([^<]+)</a>'
    desc_pattern = r'<div[^>]*class="[^"]*result-content[^"]*"[^>]*>([^<]+)</div>'

    titles = re.findall(title_pattern, html)
    descriptions = re.findall(desc_pattern, html)

    for i, (url, title) in enumerate(titles[:5]):
        # Clean title
        title = re.sub(r'<[^>]+>', '', title).strip()
        title = title.replace('<em>', '').replace('</em>', '')

        # Get description if available
        desc = descriptions[i] if i < len(descriptions) else ""
        desc = re.sub(r'<[^>]+>', '', desc).strip()
        desc = desc.replace('<em>', '').replace('</em>', '')

        # Make URL absolute
        if url.startswith("//"):
            url = "https:" + url
        elif url.startswith("/"):
            url = "https://baike.baidu.com" + url

        results.append({
            "title": title,
            "url": url,
            "snippet": desc[:300] if desc else "",
            "source": "百度百科"
        })

    # If no results found with pattern, try alternative
    if not results:
        # Try direct lookup pattern
        direct_pattern = r'<a[^>]*href="(https://baike\.baidu\.com/item/[^"]+)"[^>]*>([^<]+)</a>'
        direct_matches = re.findall(direct_pattern, html)
        for url, title in direct_matches[:5]:
            title = re.sub(r'<[^>]+>', '', title).strip()
            results.append({
                "title": title,
                "url": url,
                "snippet": "",
                "source": "百度百科"
            })

    return results


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    query = sys.argv[1]
    result = search_baidu_baike(query)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
