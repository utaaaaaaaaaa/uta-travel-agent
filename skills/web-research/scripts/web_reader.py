#!/usr/bin/env python3
"""
Web Reader Script - Fetch and extract content from any URL

Usage:
    python web_reader.py <url>

Examples:
    python web_reader.py "https://www.kiyomizudera.or.jp/"
    python web_reader.py "https://example.com/article"

Environment:
    HTTP_PROXY / HTTPS_PROXY - Optional proxy settings
"""

import json
import os
import sys
import urllib.request
import urllib.error


def read_url(url: str, timeout: int = 30) -> dict:
    """
    Fetch content from a URL.

    Args:
        url: The URL to fetch
        timeout: Request timeout in seconds

    Returns:
        Dict with 'success' and 'content' or 'error'
    """
    # Setup proxy if available
    proxies = {}
    http_proxy = os.environ.get("HTTP_PROXY") or os.environ.get("http_proxy")
    https_proxy = os.environ.get("HTTPS_PROXY") or os.environ.get("https_proxy")

    if http_proxy:
        proxies["http"] = http_proxy
    if https_proxy:
        proxies["https"] = https_proxy

    try:
        req = urllib.request.Request(
            url,
            headers={
                "User-Agent": "Mozilla/5.0 (compatible; UTA-TravelAgent/1.0)",
                "Accept": "text/html,application/xhtml+xml,text/plain",
            }
        )

        with urllib.request.urlopen(req, timeout=timeout) as response:
            content_type = response.headers.get("Content-Type", "")
            raw_content = response.read()

            # Try to decode as text
            try:
                if "charset=" in content_type:
                    charset = content_type.split("charset=")[1].split(";")[0].strip()
                    content = raw_content.decode(charset)
                else:
                    # Try common encodings
                    for encoding in ["utf-8", "gbk", "gb2312", "shift-jis", "latin-1"]:
                        try:
                            content = raw_content.decode(encoding)
                            break
                        except:
                            continue
                    else:
                        content = raw_content.decode("utf-8", errors="replace")
            except Exception as e:
                content = raw_content.decode("utf-8", errors="replace")

            # Basic HTML to text conversion
            if "text/html" in content_type:
                content = html_to_text(content)

            return {
                "success": True,
                "url": url,
                "content_type": content_type,
                "content": content[:10000],  # Limit to 10KB
                "content_length": len(content),
            }

    except urllib.error.HTTPError as e:
        return {
            "success": False,
            "error": f"HTTP Error {e.code}: {e.reason}",
            "url": url
        }
    except urllib.error.URLError as e:
        return {
            "success": False,
            "error": f"URL Error: {e.reason}",
            "url": url
        }
    except Exception as e:
        return {
            "success": False,
            "error": str(e),
            "url": url
        }


def html_to_text(html: str) -> str:
    """Simple HTML to text conversion."""
    import re

    # Remove scripts and styles
    html = re.sub(r'<script[^>]*>.*?</script>', '', html, flags=re.DOTALL | re.IGNORECASE)
    html = re.sub(r'<style[^>]*>.*?</style>', '', html, flags=re.DOTALL | re.IGNORECASE)

    # Remove comments
    html = re.sub(r'<!--.*?-->', '', html, flags=re.DOTALL)

    # Convert common block elements to newlines
    for tag in ['p', 'div', 'br', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'li', 'tr']:
        html = re.sub(f'<{tag}[^>]*>', '\n', html, flags=re.IGNORECASE)
        html = re.sub(f'</{tag}>', '\n', html, flags=re.IGNORECASE)

    # Remove remaining tags
    html = re.sub(r'<[^>]+>', '', html)

    # Decode HTML entities
    html = html.replace('&nbsp;', ' ')
    html = html.replace('&amp;', '&')
    html = html.replace('&lt;', '<')
    html = html.replace('&gt;', '>')
    html = html.replace('&quot;', '"')

    # Clean up whitespace
    html = re.sub(r'\n\s*\n', '\n\n', html)
    html = re.sub(r' +', ' ', html)

    return html.strip()


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    url = sys.argv[1]
    result = read_url(url)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
