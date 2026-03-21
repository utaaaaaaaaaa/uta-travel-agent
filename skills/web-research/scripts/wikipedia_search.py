#!/usr/bin/env python3
"""
Wikipedia Search Script

Usage:
    python wikipedia_search.py "query" [--lang en|zh]

Examples:
    python wikipedia_search.py "清水寺" --lang zh
    python wikipedia_search.py "Kyoto temples" --lang en
"""

import json
import os
import sys
import urllib.parse
import urllib.request


def get_proxy_handler():
    """Get proxy handler if HTTP_PROXY is set."""
    http_proxy = os.environ.get("HTTP_PROXY") or os.environ.get("http_proxy")
    https_proxy = os.environ.get("HTTPS_PROXY") or os.environ.get("https_proxy")

    if http_proxy or https_proxy:
        proxies = {}
        if http_proxy:
            proxies["http"] = http_proxy
        if https_proxy:
            proxies["https"] = https_proxy
        return urllib.request.ProxyHandler(proxies)
    return None


def search_wikipedia(query: str, lang: str = "en", limit: int = 3) -> dict:
    """
    Search Wikipedia and return results.

    Args:
        query: Search query
        lang: Language code (en, zh, ja, etc.)
        limit: Max results to return

    Returns:
        Dict with 'success' and 'results' or 'error'
    """
    base_url = f"https://{lang}.wikipedia.org/w/api.php"

    # Search for pages
    search_params = {
        "action": "query",
        "list": "search",
        "srsearch": query,
        "format": "json",
        "srlimit": limit,
    }

    try:
        # Setup opener with proxy if available
        proxy_handler = get_proxy_handler()
        if proxy_handler:
            opener = urllib.request.build_opener(proxy_handler)
        else:
            opener = urllib.request.build_opener()

        url = f"{base_url}?{urllib.parse.urlencode(search_params)}"
        with opener.open(url, timeout=15) as response:
            data = json.loads(response.read().decode())

        results = []
        for item in data.get("query", {}).get("search", []):
            # Get extract for each page
            extract_params = {
                "action": "query",
                "prop": "extracts",
                "exintro": True,
                "explaintext": True,
                "pageids": item["pageid"],
                "format": "json",
                "exsentences": 5,
            }

            extract_url = f"{base_url}?{urllib.parse.urlencode(extract_params)}"
            with opener.open(extract_url, timeout=15) as extract_response:
                extract_data = json.loads(extract_response.read().decode())

            page = extract_data.get("query", {}).get("pages", {}).get(str(item["pageid"]), {})
            extract = page.get("extract", "")

            results.append({
                "title": item["title"],
                "snippet": item.get("snippet", ""),
                "extract": extract,
                "url": f"https://{lang}.wikipedia.org/wiki/{urllib.parse.quote(item['title'])}",
            })

        return {"success": True, "results": results}

    except Exception as e:
        return {"success": False, "error": str(e)}


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    query = sys.argv[1]
    lang = "en"

    # Parse arguments
    args = sys.argv[2:]
    if "--lang" in args:
        idx = args.index("--lang")
        if idx + 1 < len(args):
            lang = args[idx + 1]

    result = search_wikipedia(query, lang)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
