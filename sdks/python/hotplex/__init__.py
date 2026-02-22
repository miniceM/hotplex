"""
HotPlex Python SDK

A production-ready Python client for the HotPlex AI Agent Control Plane.
"""

__version__ = "0.1.0"
__author__ = "HotPlex Team"

from .client import HotPlexClient
from .opencode import OpenCodeClient
from .errors import (
    HotPlexError,
    ConnectionError,
    TimeoutError,
    ExecutionError,
    DangerBlockedError,
    SessionError,
)
from .events import Event, EventType
from .config import Config, ClientConfig

__all__ = [
    "HotPlexClient",
    "OpenCodeClient",
    "HotPlexError",
    "ConnectionError",
    "TimeoutError",
    "ExecutionError",
    "DangerBlockedError",
    "SessionError",
    "Event",
    "EventType",
    "Config",
    "ClientConfig",
]
