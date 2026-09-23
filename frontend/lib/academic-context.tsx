"use client";

import { createContext, useContext, useEffect, useMemo, useState, ReactNode } from "react";

export type AcademicSession = {
  id: string;
  name: string;
  start_date: string;
  end_date: string;
  status: string;
  terms?: AcademicTerm[];
};

export type AcademicTerm = {
  id: string;
  academic_session_id: string;
  name: string;
  start_date: string;
  end_date: string;
  status: string;
};

type AcademicSelection = {
  sessionId: string;
  termId: string;
};

type AcademicContextValue = AcademicSelection & {
  sessions: AcademicSession[];
  terms: AcademicTerm[];
  loading: boolean;
  error: string;
  setSession: (id: string) => void;
  setTerm: (id: string) => void;
  refresh: () => Promise<void>;
};

const STORAGE_KEY = "stonez.academic.selection";
function scopedStorageKey(){if(typeof window==="undefined")return STORAGE_KEY;try{const token=localStorage.getItem("access_token")||"";const payload=token.split(".")[1];if(!payload)return STORAGE_KEY;const claims=JSON.parse(atob(payload.replace(/-/g,"+").replace(/_/g,"/")));return STORAGE_KEY+":"+String(claims.sub||claims.user_id||"current")+":"+String(claims.school_id||"school")}catch{return STORAGE_KEY}}
const AcademicContext = createContext<AcademicContextValue | null>(null);

function list<T>(value: any, key: string): T[] {
  return Array.isArray(value) ? value : value?.[key] || [];
}

export function AcademicContextProvider({ children }: { children: ReactNode }) {
  const [sessions, setSessions] = useState<AcademicSession[]>([]);
  const [sessionId, setSessionId] = useState("");
  const [termId, setTermId] = useState("");
  const [terms, setTerms] = useState<AcademicTerm[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  async function refresh() {
    const token = localStorage.getItem("access_token");
    if (!token) {
      setLoading(false);
      return;
    }
    try {
      setError("");
      const response = await fetch("/backend/academic-context", {
        headers: { Authorization: "Bearer " + token },
      });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) throw new Error(data?.error?.message || "Unable to load academic context");

      const nextSessions = list<AcademicSession>(data, "sessions");
      setSessions(nextSessions);

      const saved = JSON.parse(localStorage.getItem(scopedStorageKey()) || "null") as AcademicSelection | null;
      const savedSession = saved && nextSessions.some(s => s.id === saved.sessionId) ? saved.sessionId : "";
      const nextSession = savedSession || nextSessions.find(s => s.status === "active")?.id || nextSessions[0]?.id || "";
      setSessionId(nextSession);

      const selectedSession = nextSessions.find(s => s.id === nextSession);
      const nextTerms = selectedSession?.terms || [];
      setTerms(nextTerms);
      const savedTerm = saved && nextTerms.some(t => t.id === saved.termId) ? saved.termId : "";
      setTermId(savedTerm || nextTerms.find(t => t.status === "active")?.id || nextTerms[0]?.id || "");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to load academic context");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  useEffect(() => {
    const selected = sessions.find(s => s.id === sessionId);
    const nextTerms = selected?.terms || [];
    setTerms(nextTerms);
    if (termId && nextTerms.some(t => t.id === termId)) return;
    setTermId(nextTerms.find(t => t.status === "active")?.id || nextTerms[0]?.id || "");
  }, [sessionId, sessions]);

  useEffect(() => {
    if (!sessionId) return;
    localStorage.setItem(scopedStorageKey(), JSON.stringify({ sessionId, termId }));
    window.dispatchEvent(new CustomEvent("stonez:academic-context-changed", { detail: { sessionId, termId } }));
  }, [sessionId, termId]);

  const value = useMemo(() => ({
    sessionId, termId, sessions, terms, loading, error,
    setSession: setSessionId,
    setTerm: setTermId,
    refresh,
  }), [sessionId, termId, sessions, terms, loading, error]);

  return <AcademicContext.Provider value={value}>{children}</AcademicContext.Provider>;
}

export function useAcademicContext() {
  const value = useContext(AcademicContext);
  if (!value) throw new Error("useAcademicContext must be used inside AcademicContextProvider");
  return value;
}

export function getAcademicSelection(): AcademicSelection | null {
  if (typeof window === "undefined") return null;
  try {
    return JSON.parse(localStorage.getItem(scopedStorageKey()) || "null");
  } catch {
    return null;
  }
}
