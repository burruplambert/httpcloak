"""
httpcloak - Browser fingerprint emulation HTTP client

This library provides HTTP/1.1, HTTP/2, and HTTP/3 requests with accurate
browser TLS and HTTP fingerprints to bypass bot detection.

Example:
    from httpcloak import Session

    # Sync usage
    session = Session(preset="chrome-143")
    response = session.get("https://example.com")
    print(response.text)

    # Async usage
    import asyncio
    async def main():
        session = Session(preset="chrome-143")
        response = await session.get_async("https://example.com")
        print(response.text)
    asyncio.run(main())
"""

from .client import Session, Response, HTTPCloakError, available_presets, version

__all__ = ["Session", "Response", "HTTPCloakError", "available_presets", "version"]
__version__ = "1.0.0"
