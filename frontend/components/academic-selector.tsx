"use client";
import { useAcademicContext } from "../lib/academic-context";

export default function AcademicSelector() {
  const { sessions, terms, sessionId, termId, setSession, setTerm, loading, error } = useAcademicContext();
  return <div className="panel academic-context-panel">
    <div className="panel-head"><div><h2>Academic context</h2><p>Choose the academic session and term used across school workflows.</p></div>{error&&<span className="pill withdrawn">{error}</span>}</div>
    <div className="form-grid">
      <label>Academic session<select value={sessionId} onChange={e=>setSession(e.target.value)} disabled={loading||!sessions.length}><option value="">Select academic session</option>{sessions.map(s=><option key={s.id} value={s.id}>{s.name}{s.status==="active"?" · Current":""}</option>)}</select></label>
      <label>Term<select value={termId} onChange={e=>setTerm(e.target.value)} disabled={loading||!sessionId||!terms.length}><option value="">Select term</option>{terms.map(t=><option key={t.id} value={t.id}>{t.name}{t.status==="active"?" · Current":""}</option>)}</select></label>
    </div>
  </div>;
}
