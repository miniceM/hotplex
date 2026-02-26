#!/usr/bin/env python3
"""
Claude Code /clear Command Implementation

ARCHITECTURE:
The /clear command performs a FORCE RESET by:
1. Reset ProviderSessionID (marks session for new random UUID on restart)
2. Delete HotPlex session marker (~/.hotplex/sessions/{sessionID}.lock)
3. Terminate the current session process

HOW IT WORKS:
- ProviderSessionID is normally deterministic: SHA1(namespace:sessionID)
- /clear marks the session in resetSessions map
- Next message triggers startSession() which checks resetSessions
- If marked, generates random UUID instead of SHA1
- Claude Code sees new ProviderSessionID → starts fresh session

COMPARISON:
| Command | ProviderSessionID | Marker | Process | Next Message |
|---------|------------------|--------|---------|--------------|
| Normal  | Same (SHA1)      | Keep   | Keep    | Resume       |
| /dc     | Same (SHA1)      | Keep   | Kill    | Resume       |
| /clear  | NEW (random)     | Delete | Kill    | COLD START   |

USAGE:
    In Slack: /clear
    
    This will:
    1. Mark session for ProviderSessionID reset
    2. Delete the HotPlex marker file
    3. Kill the Claude Code process
    4. Next message creates brand new session with new ProviderSessionID
"""

print(__doc__)

# Test the flow
import subprocess
import json
import time

def test_flow():
    print("\n" + "="*60)
    print("TEST FLOW (manual verification in Slack):")
    print("="*60)
    
    print("""
1. In Slack, send: "记住：ping 回复 hi"
   → Claude Code remembers this
    
2. Send: "ping"
   → Should respond: "hi"
   
3. Send: /clear
   → ProviderSessionID is reset
   → Session is terminated
   
4. Send: "ping"
   → New session with new ProviderSessionID
   → Should NOT remember "ping 回复 hi"
   → Responds normally (not "hi")
    """)

if __name__ == "__main__":
    test_flow()
