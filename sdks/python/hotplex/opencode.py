"""OpenCode HTTP/SSE Client for HotPlex"""

import json
import logging
import threading
import requests
from typing import Optional, Generator
from .config import Config, ClientConfig
from .events import Event, EventType

try:
    import sseclient
except ImportError:
    sseclient = None

logger = logging.getLogger("hotplex.opencode")

class OpenCodeClient:
    """
    Python client for HotPlex OpenCode compatibility layer.
    Uses HTTP for commands and SSE for events.
    """

    def __init__(self, config: Optional[ClientConfig] = None):
        if sseclient is None:
            raise ImportError(
                "sseclient-py is required. Install with: pip install sseclient-py"
            )
        self.config = config or ClientConfig()
        self.session = requests.Session()
        if self.config.api_key:
            self.session.headers.update({"Authorization": f"Bearer {self.config.api_key}"})
        
        # Base URL adjustment from ws:// to http:// if needed
        self.base_url = self.config.url.replace("ws://", "http://").replace("wss://", "https://")
        if "/ws/v1/agent" in self.base_url:
             self.base_url = self.base_url.replace("/ws/v1/agent", "")

    def create_session(self) -> str:
        """Create a new session and return its ID."""
        resp = self.session.post(f"{self.base_url}/session")
        resp.raise_for_status()
        data = resp.json()
        return data["info"]["id"]

    def execute_stream(
        self,
        prompt: str,
        config: Optional[Config] = None,
    ) -> Generator[Event, None, None]:
        """
        Execute a prompt and yield events via SSE.
        Note: In OpenCode protocol, we listen to a global Event Stream.
        """
        session_id = config.session_id if config else self.create_session()
        
        # 1. Start listening to SSE in a separate connection/generator
        sse_url = f"{self.base_url}/global/event"
        
        # 2. Send the prompt
        prompt_url = f"{self.base_url}/session/{session_id}/message"
        
        # We need to be listening BEFORE sending the prompt or have a way to match.
        # OpenCode usually has a persistent SSE connection.
        
        response = self.session.get(sse_url, stream=True)
        client = sseclient.SSEClient(response)
        
        # Send prompt in a separate thread or just before starting to iterate
        def send_prompt():
            try:
                self.session.post(prompt_url, json={"prompt": prompt})
            except Exception as e:
                logger.error(f"Failed to send prompt: {e}")

        # In a real scenario, we might want to start listening first.
        # But SSE buffer might catch it if we are fast.
        threading.Thread(target=send_prompt, daemon=True).start()

        for msg in client.events():
            if not msg.data:
                continue
            
            raw_data = json.loads(msg.data)
            # Map OpenCode Part to HotPlex Event
            event = self._map_opencode_to_event(raw_data, session_id)
            if event:
                yield event
                if event.type in [EventType.SESSION_STATS, EventType.ERROR]:
                    break

    def _map_opencode_to_event(self, data: dict, target_session_id: str) -> Optional[Event]:
        """Maps OpenCode's message.part.updated to HotPlex Event"""
        payload = data.get("payload", {})
        if payload.get("type") != "message.part.updated":
            return None
        
        properties = payload.get("properties", {})
        part = properties.get("part", {})
        
        # Only interested in our session
        if part.get("sessionID") != target_session_id:
            return None
        
        part_type = part.get("type")
        event_type = None
        event_data = None
        
        if part_type == "text":
            event_type = EventType.ANSWER
            event_data = part.get("text")
        elif part_type == "reasoning":
            event_type = EventType.THINKING
            event_data = part.get("text")
        elif part_type == "tool":
            state = part.get("state", {})
            status = state.get("status")
            if status == "running":
                event_type = EventType.TOOL_USE
                event_data = part.get("tool") # This depends on how server handles it
            elif status == "completed":
                event_type = EventType.TOOL_RESULT
                event_data = state.get("output")
        
        if event_type:
            return Event(
                type=event_type,
                data=event_data,
                session_id=target_session_id,
                timestamp=None # Should extract if available
            )
        return None
