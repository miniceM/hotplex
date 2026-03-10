import requests
import json
import threading
import sys

# Configuration
BASE_URL = "http://localhost:8080"

def listen_to_events():
    """Listens to the SSE global event stream."""
    print("📡 Connecting to SSE event stream...")
    try:
        response = requests.get(f"{BASE_URL}/global/event", stream=True, timeout=None)
        
        for line in response.iter_lines():
            if line:
                decoded_line = line.decode('utf-8')
                if decoded_line.startswith('data: '):
                    data_str = decoded_line[len('data: '):]
                    try:
                        event = json.loads(data_str)
                        # Process OpenCode event parts
                        payload = event.get("payload", {})
                        p_type = payload.get("type")
                        props = payload.get("properties", {})
                        
                        if p_type == "message.part.updated":
                            part = props.get("part", {})
                            content_type = part.get("type")
                            
                            if content_type == "text":
                                print(f"\n🤖: {part.get('text')}", end="", flush=True)
                            elif content_type == "reasoning":
                                print(f"\n🤔 Thinking: {part.get('text')}", end="", flush=True)
                            elif content_type == "tool":
                                state = part.get("state", {})
                                status = state.get("status")
                                if status == "running":
                                    print(f"\n🛠️ Using Tool: {part.get('tool')} (Input: {state.get('input')})")
                                elif status == "completed":
                                    print(f"✅ Tool Result: {state.get('output')[:100]}...")
                        elif p_type == "server.connected":
                            print("✅ Connected to HotPlex OpenCode Server")
                    except json.JSONDecodeError:
                        print(f"⚠️ Failed to decode: {data_str}")
    except Exception as e:
        print(f"\n❌ SSE Connection lost: {e}")

def create_session():
    """Creates a new HotPlex session via OpenCode API."""
    print("🆕 Creating session...")
    resp = requests.post(f"{BASE_URL}/session")
    resp.raise_for_status()
    session_id = resp.json()["info"]["id"]
    print(f"✅ Session Created: {session_id}")
    return session_id

def send_prompt(session_id, prompt, system_prompt=None):
    """Sends a prompt to an active session, optionally with a system prompt."""
    print(f"\n👤 Sending prompt: {prompt}")
    if system_prompt:
        print(f"📖 System Prompt: {system_prompt}")
    url = f"{BASE_URL}/session/{session_id}/message"
    payload = {"prompt": prompt}
    if system_prompt:
        payload["system_prompt"] = system_prompt
    resp = requests.post(url, json=payload)
    resp.raise_for_status()
    print("📤 Prompt accepted")

if __name__ == "__main__":
    # 1. Start event listener in background
    listener_thread = threading.Thread(target=listen_to_events, daemon=True)
    listener_thread.start()
    
    try:
        # 2. Setup session
        sid = create_session()
        
        # 3. Interactive Loop or Single Prompt with System Prompt injection
        prompt = "Write a basic hello world in Python"
        system_prompt = "You are an expert Python developer. Use type hints and docstrings."
        send_prompt(sid, prompt, system_prompt=system_prompt)
        
        # Keep main thread alive to watch results
        print("\n⏳ Waiting for AI results (Ctrl+C to exit)...")
        while True:
            pass
    except KeyboardInterrupt:
        print("\n👋 Exiting...")
        sys.exit(0)
    except Exception as e:
        print(f"\n❌ Error: {e}")
