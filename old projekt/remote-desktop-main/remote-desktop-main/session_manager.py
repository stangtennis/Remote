import datetime
import logging
import uuid
from typing import Dict, Set, Optional

logger = logging.getLogger(__name__)

class Session:
    def __init__(self, client_sid: str):
        self.client_sid = client_sid
        self.controllers: Set[str] = set()
        self.start_time = datetime.datetime.now()

class SessionManager:
    def __init__(self):
        self._sessions: Dict[str, Session] = {}

    def create_session(self, client_sid: str) -> str:
        """Create a new session for a client"""
        session_id = str(uuid.uuid4())[:8]
        self._sessions[session_id] = Session(client_sid)
        return session_id

    def get_session(self, session_id: str) -> Optional[Session]:
        """Get session by ID"""
        return self._sessions.get(session_id)

    def add_controller(self, session_id: str, controller_sid: str) -> bool:
        """Add a controller to a session"""
        session = self.get_session(session_id)
        if session:
            session.controllers.add(controller_sid)
            return True
        return False

    def remove_controller(self, session_id: str, controller_sid: str) -> bool:
        """Remove a controller from a session"""
        session = self.get_session(session_id)
        if session and controller_sid in session.controllers:
            session.controllers.remove(controller_sid)
            return True
        return False

    def remove_session_by_client(self, client_sid: str) -> Optional[str]:
        """Remove session by client SID and return session ID if found"""
        for session_id, session in list(self._sessions.items()):
            if session.client_sid == client_sid:
                del self._sessions[session_id]
                return session_id
        return None

    def remove_session(self, session_id: str) -> bool:
        """Remove session by session ID"""
        if session_id in self._sessions:
            del self._sessions[session_id]
            return True
        return False

    def get_all_session_ids(self) -> list:
        """Get list of all active session IDs"""
        return list(self._sessions.keys())

    def get_session_by_client(self, client_sid: str) -> Optional[str]:
        """Get session ID by client SID"""
        for session_id, session in self._sessions.items():
            if session.client_sid == client_sid:
                return session_id
        return None

    def cleanup_stale_sessions(self, max_age_minutes: int = 60) -> list:
        """Remove sessions older than max_age_minutes"""
        now = datetime.datetime.now()
        stale_sessions = []
        for session_id, session in list(self._sessions.items()):
            if (now - session.start_time).total_seconds() > max_age_minutes * 60:
                del self._sessions[session_id]
                stale_sessions.append(session_id)
        return stale_sessions
