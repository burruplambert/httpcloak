"""
HTTPCloak Python Client

Provides sync and async HTTP client with browser fingerprint emulation.
"""

import asyncio
import json
import os
import platform
from ctypes import c_char_p, c_int64, cdll
from dataclasses import dataclass
from pathlib import Path
from threading import Lock
from typing import Dict, List, Optional, Union


class HTTPCloakError(Exception):
    """Base exception for HTTPCloak errors."""
    pass


@dataclass
class Response:
    """HTTP Response object."""
    status_code: int
    headers: Dict[str, str]
    body: bytes
    text: str
    final_url: str
    protocol: str

    @classmethod
    def from_json(cls, data: dict) -> "Response":
        body = data.get("body", "")
        if isinstance(body, str):
            body_bytes = body.encode("utf-8")
        else:
            body_bytes = body
        return cls(
            status_code=data.get("status_code", 0),
            headers=data.get("headers", {}),
            body=body_bytes,
            text=body if isinstance(body, str) else body.decode("utf-8", errors="replace"),
            final_url=data.get("final_url", ""),
            protocol=data.get("protocol", ""),
        )


def _get_lib_path() -> str:
    """Get the path to the shared library based on platform."""
    system = platform.system().lower()
    machine = platform.machine().lower()

    # Normalize architecture names
    if machine in ("x86_64", "amd64"):
        arch = "amd64"
    elif machine in ("aarch64", "arm64"):
        arch = "arm64"
    else:
        arch = machine

    # Determine file extension
    if system == "darwin":
        ext = ".dylib"
        os_name = "darwin"
    elif system == "windows":
        ext = ".dll"
        os_name = "windows"
    else:
        ext = ".so"
        os_name = "linux"

    lib_name = f"libhttpcloak-{os_name}-{arch}{ext}"

    # Search paths
    search_paths = [
        # Same directory as this file
        Path(__file__).parent / lib_name,
        Path(__file__).parent / "lib" / lib_name,
        # Package data
        Path(__file__).parent.parent / "lib" / lib_name,
        # System paths
        Path(f"/usr/local/lib/{lib_name}"),
        Path(f"/usr/lib/{lib_name}"),
    ]

    # Check HTTPCLOAK_LIB_PATH environment variable
    env_path = os.environ.get("HTTPCLOAK_LIB_PATH")
    if env_path:
        search_paths.insert(0, Path(env_path))

    for path in search_paths:
        if path.exists():
            return str(path)

    raise HTTPCloakError(
        f"Could not find httpcloak library ({lib_name}). "
        f"Set HTTPCLOAK_LIB_PATH environment variable or install the library."
    )


# Load the shared library
_lib = None
_lib_lock = Lock()


def _get_lib():
    """Get or load the shared library."""
    global _lib
    if _lib is None:
        with _lib_lock:
            if _lib is None:
                lib_path = _get_lib_path()
                _lib = cdll.LoadLibrary(lib_path)
                _setup_lib(_lib)
    return _lib


def _setup_lib(lib):
    """Setup function signatures for the library."""
    # Session management
    lib.httpcloak_session_new.argtypes = [c_char_p]
    lib.httpcloak_session_new.restype = c_int64

    lib.httpcloak_session_free.argtypes = [c_int64]
    lib.httpcloak_session_free.restype = None

    # Sync requests
    lib.httpcloak_get.argtypes = [c_int64, c_char_p, c_char_p]
    lib.httpcloak_get.restype = c_char_p

    lib.httpcloak_post.argtypes = [c_int64, c_char_p, c_char_p, c_char_p]
    lib.httpcloak_post.restype = c_char_p

    lib.httpcloak_request.argtypes = [c_int64, c_char_p]
    lib.httpcloak_request.restype = c_char_p

    # Cookies
    lib.httpcloak_get_cookies.argtypes = [c_int64]
    lib.httpcloak_get_cookies.restype = c_char_p

    lib.httpcloak_set_cookie.argtypes = [c_int64, c_char_p, c_char_p]
    lib.httpcloak_set_cookie.restype = None

    # Utility
    lib.httpcloak_free_string.argtypes = [c_char_p]
    lib.httpcloak_free_string.restype = None

    lib.httpcloak_version.argtypes = []
    lib.httpcloak_version.restype = c_char_p

    lib.httpcloak_available_presets.argtypes = []
    lib.httpcloak_available_presets.restype = c_char_p


def _parse_response(result: bytes) -> Response:
    """Parse JSON response from library."""
    if result is None:
        raise HTTPCloakError("No response received")

    data = json.loads(result.decode("utf-8"))

    if "error" in data:
        raise HTTPCloakError(data["error"])

    return Response.from_json(data)


def version() -> str:
    """Get the httpcloak library version."""
    lib = _get_lib()
    result = lib.httpcloak_version()
    return result.decode("utf-8") if result else "unknown"


def available_presets() -> List[str]:
    """Get list of available browser presets."""
    lib = _get_lib()
    result = lib.httpcloak_available_presets()
    if result:
        return json.loads(result.decode("utf-8"))
    return []


class Session:
    """
    HTTP Session with browser fingerprint emulation.

    Args:
        preset: Browser preset to use (default: "chrome-143")
        proxy: Proxy URL (e.g., "http://user:pass@host:port")
        timeout: Request timeout in seconds (default: 30)

    Example:
        session = Session(preset="chrome-143")
        response = session.get("https://example.com")
        print(response.status_code, response.text)
    """

    def __init__(
        self,
        preset: str = "chrome-143",
        proxy: Optional[str] = None,
        timeout: int = 30,
    ):
        self._lib = _get_lib()

        config = {
            "preset": preset,
            "timeout": timeout,
        }
        if proxy:
            config["proxy"] = proxy

        config_json = json.dumps(config).encode("utf-8")
        self._handle = self._lib.httpcloak_session_new(config_json)

        if self._handle == 0:
            raise HTTPCloakError("Failed to create session")

    def __del__(self):
        self.close()

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()

    def close(self):
        """Close the session and release resources."""
        if hasattr(self, "_handle") and self._handle:
            self._lib.httpcloak_session_free(self._handle)
            self._handle = 0

    # =========================================================================
    # Synchronous Methods
    # =========================================================================

    def get(
        self,
        url: str,
        headers: Optional[Dict[str, str]] = None,
    ) -> Response:
        """
        Perform a GET request.

        Args:
            url: Request URL
            headers: Optional custom headers

        Returns:
            Response object
        """
        headers_json = json.dumps(headers).encode("utf-8") if headers else None
        result = self._lib.httpcloak_get(
            self._handle,
            url.encode("utf-8"),
            headers_json,
        )
        return _parse_response(result)

    def post(
        self,
        url: str,
        body: Union[str, bytes, dict, None] = None,
        headers: Optional[Dict[str, str]] = None,
    ) -> Response:
        """
        Perform a POST request.

        Args:
            url: Request URL
            body: Request body (string, bytes, or dict for JSON)
            headers: Optional custom headers

        Returns:
            Response object
        """
        if isinstance(body, dict):
            body = json.dumps(body)
            if headers is None:
                headers = {}
            headers.setdefault("Content-Type", "application/json")

        if isinstance(body, str):
            body = body.encode("utf-8")

        headers_json = json.dumps(headers).encode("utf-8") if headers else None
        result = self._lib.httpcloak_post(
            self._handle,
            url.encode("utf-8"),
            body,
            headers_json,
        )
        return _parse_response(result)

    def request(
        self,
        method: str,
        url: str,
        headers: Optional[Dict[str, str]] = None,
        body: Union[str, bytes, dict, None] = None,
        timeout: Optional[int] = None,
    ) -> Response:
        """
        Perform a custom HTTP request.

        Args:
            method: HTTP method (GET, POST, PUT, DELETE, etc.)
            url: Request URL
            headers: Optional custom headers
            body: Optional request body
            timeout: Optional request timeout in seconds

        Returns:
            Response object
        """
        if isinstance(body, dict):
            body = json.dumps(body)
            if headers is None:
                headers = {}
            headers.setdefault("Content-Type", "application/json")

        if isinstance(body, bytes):
            body = body.decode("utf-8")

        request_config = {
            "method": method.upper(),
            "url": url,
        }
        if headers:
            request_config["headers"] = headers
        if body:
            request_config["body"] = body
        if timeout:
            request_config["timeout"] = timeout

        result = self._lib.httpcloak_request(
            self._handle,
            json.dumps(request_config).encode("utf-8"),
        )
        return _parse_response(result)

    def put(self, url: str, body=None, headers=None) -> Response:
        """Perform a PUT request."""
        return self.request("PUT", url, headers=headers, body=body)

    def delete(self, url: str, headers=None) -> Response:
        """Perform a DELETE request."""
        return self.request("DELETE", url, headers=headers)

    def patch(self, url: str, body=None, headers=None) -> Response:
        """Perform a PATCH request."""
        return self.request("PATCH", url, headers=headers, body=body)

    def head(self, url: str, headers=None) -> Response:
        """Perform a HEAD request."""
        return self.request("HEAD", url, headers=headers)

    def options(self, url: str, headers=None) -> Response:
        """Perform an OPTIONS request."""
        return self.request("OPTIONS", url, headers=headers)

    # =========================================================================
    # Asynchronous Methods
    # =========================================================================

    async def get_async(
        self,
        url: str,
        headers: Optional[Dict[str, str]] = None,
    ) -> Response:
        """
        Perform an async GET request.

        Args:
            url: Request URL
            headers: Optional custom headers

        Returns:
            Response object
        """
        loop = asyncio.get_event_loop()
        return await loop.run_in_executor(None, lambda: self.get(url, headers))

    async def post_async(
        self,
        url: str,
        body: Union[str, bytes, dict, None] = None,
        headers: Optional[Dict[str, str]] = None,
    ) -> Response:
        """
        Perform an async POST request.

        Args:
            url: Request URL
            body: Request body
            headers: Optional custom headers

        Returns:
            Response object
        """
        loop = asyncio.get_event_loop()
        return await loop.run_in_executor(None, lambda: self.post(url, body, headers))

    async def request_async(
        self,
        method: str,
        url: str,
        headers: Optional[Dict[str, str]] = None,
        body: Union[str, bytes, dict, None] = None,
        timeout: Optional[int] = None,
    ) -> Response:
        """
        Perform an async custom HTTP request.

        Args:
            method: HTTP method
            url: Request URL
            headers: Optional custom headers
            body: Optional request body
            timeout: Optional request timeout

        Returns:
            Response object
        """
        loop = asyncio.get_event_loop()
        return await loop.run_in_executor(
            None, lambda: self.request(method, url, headers, body, timeout)
        )

    # =========================================================================
    # Cookie Management
    # =========================================================================

    def get_cookies(self) -> Dict[str, str]:
        """Get all cookies from the session."""
        result = self._lib.httpcloak_get_cookies(self._handle)
        if result:
            return json.loads(result.decode("utf-8"))
        return {}

    def set_cookie(self, name: str, value: str):
        """Set a cookie in the session."""
        self._lib.httpcloak_set_cookie(
            self._handle,
            name.encode("utf-8"),
            value.encode("utf-8"),
        )

    @property
    def cookies(self) -> Dict[str, str]:
        """Get cookies as a property."""
        return self.get_cookies()
