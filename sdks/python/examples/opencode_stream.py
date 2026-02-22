"""Example: OpenCode HTTP/SSE Stream with Python SDK"""

import sys
import os

# Add parent directory to sys.path to import hotplex
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from hotplex import OpenCodeClient, Config

def main():
    # 1. Initialize OpenCode client (defaults to http://localhost:8080)
    client = OpenCodeClient()

    print("🚀 Starting OpenCode stream...")
    
    # 2. Execute a task
    config = Config(
        session_id="python-opencode-test",
        work_dir="/tmp/hotplex-python"
    )

    try:
        for event in client.execute_stream(
            prompt="Write a Hello World program in Python.",
            config=config
        ):
            if event.type == "answer":
                print(event.data, end="", flush=True)
            elif event.type == "thinking":
                print(f"\n[Thinking] {event.data}")
            elif event.type == "tool_use":
                print(f"\n[Tool Use] {event.data}")
            elif event.type == "tool_result":
                print(f"\n[Tool Result] {event.data}")
                
        print("\n\n✅ Task completed!")

    except Exception as e:
        print(f"\n❌ Error: {e}")

if __name__ == "__main__":
    main()
