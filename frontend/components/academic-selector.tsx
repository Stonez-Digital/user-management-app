"use client";
import Link from "next/link";
import { useAcademicContext } from "../lib/academic-context";

export default function AcademicSelector() {
  const { sessions, terms, sessionId, termId, setSession, setTerm, loading, error } = useAcademicContext();
  return <div className="panel academic-context-panel">
    <div className="panel-head"><div><h2>Academic context</h2><p>Choose the academic session and term used across school workflows.</p></div>{error&&<span className="pill withdrawn">{error}</span>}</div>
    <div className="form-grid">
      <label>Academic session<select value={sessionId} onChange={e=>setSession(e.target.value)} disabled={loading||!sessions.length}><option value="">Select academic session</option>{sessions.map(s=><option key={s.id} value={s.id}>{s.name}{s.status==="active"?" · Current":""}</option>)}</select></label>
      <label>Term<select value={termId} onChange={e=>setTerm(e.target.value)} disabled={loading||!sessionId||!terms.length}><option value="">Select term</option>{terms.map(t=><option key={t.id} value={t.id}>{t.name}{t.status==="active"?" · Current":""}</option>)}</select></label>
    </div>
    {!loading && sessions.length === 0 && <div className="empty academic-context-empty">
      <strong>Your school calendar has not been created yet.</strong>
      <p>Create an academic session first. You can then add First, Second and Third Term with their dates.</p>
      <Link className="primary" href="/dashboard/academic">Create academic session →</Link>
    </div>}
    {!loading && sessions.length > 0 && !terms.length && <div className="empty academic-context-empty">
      <strong>No terms are configured for this session.</strong>
      <p>Add at least one term to enable term-based school workflows.</p>
      <Link className="primary" href="/dashboard/academic">Add academic term →</Link>
    </div>}
  </div>;
}
