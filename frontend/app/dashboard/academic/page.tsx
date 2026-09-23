"use client";

import { FormEvent, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

type Session = { id: string; name: string; start_date: string; end_date: string; status: string };
type Term = { id: string; academic_session_id: string; name: string; start_date: string; end_date: string; status: string };
type SchoolClass = { id: string; name: string; level: number };
type Section = { id: string; class_id: string; name: string };
type Subject = { id: string; code: string; name: string; description?: string; active: boolean };

async function api(path: string, options: RequestInit = {}) {
  const token = localStorage.getItem("access_token");
  const response = await fetch("/backend" + path, {
    ...options,
    headers: { "Content-Type": "application/json", Authorization: "Bearer " + token, ...(options.headers || {}) },
  });
  if (response.status === 401) throw new Error("Session expired");
  const data = await response.json();
  if (!response.ok) throw new Error(data?.error?.message || data?.error || "Request failed");
  return data;
}

function dateValue(value: string) {
  return value ? new Date(value).toISOString().slice(0, 10) : "";
}
function datePayload(value: string) {
  return new Date(value + "T00:00:00Z").toISOString();
}

export default function AcademicPage() {
  const router = useRouter();
  const [sessions, setSessions] = useState<Session[]>([]);
  const [terms, setTerms] = useState<Record<string, Term[]>>({});
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [sections, setSections] = useState<Record<string, Section[]>>({});
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function load() {
    try {
      const [s, c, sub] = await Promise.all([api("/admin/academic-sessions"), api("/admin/classes"), api("/admin/subjects")]);
      setSessions(Array.isArray(s) ? s : s?.sessions || []);
      setClasses(Array.isArray(c) ? c : c?.classes || []);
      setSubjects(Array.isArray(sub) ? sub : sub?.subjects || []);
      setError("");
    } catch (e) {
      const message = e instanceof Error ? e.message : "Unable to load academic data";
      setError(message);
      if (message === "Session expired") router.push("/");
    }
  }

  useEffect(() => { load(); }, [router]);

  async function createSession(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true);
    const form = new FormData(event.currentTarget);
    try {
      await api("/admin/academic-sessions", { method: "POST", body: JSON.stringify({
        name: form.get("name"), start_date: datePayload(String(form.get("start_date"))), end_date: datePayload(String(form.get("end_date"))), status: form.get("status"),
      })});
      event.currentTarget.reset(); await load();
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to create session"); }
    finally { setBusy(false); }
  }

  async function createClass(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true);
    const form = new FormData(event.currentTarget);
    try {
      await api("/admin/classes", { method: "POST", body: JSON.stringify({ name: form.get("name"), level: Number(form.get("level") || 0) })});
      event.currentTarget.reset(); await load();
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to create class"); }
    finally { setBusy(false); }
  }

  async function createSubject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true);
    const form = new FormData(event.currentTarget);
    try {
      await api("/admin/subjects", { method: "POST", body: JSON.stringify({ code: form.get("code"), name: form.get("name"), description: form.get("description") })});
      event.currentTarget.reset(); await load();
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to create subject"); }
    finally { setBusy(false); }
  }

  async function loadTerms(sessionId: string) {
    try {
      const data = await api("/admin/academic-sessions/" + sessionId + "/terms");
      setTerms(v => ({ ...v, [sessionId]: Array.isArray(data) ? data : data?.terms || [] }));
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to load terms"); }
  }

  async function createTerm(sessionId: string, event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true);
    const form = new FormData(event.currentTarget);
    try {
      await api("/admin/academic-sessions/" + sessionId + "/terms", { method: "POST", body: JSON.stringify({
        name: form.get("name"), start_date: datePayload(String(form.get("start_date"))), end_date: datePayload(String(form.get("end_date"))), status: form.get("status"),
      })});
      event.currentTarget.reset(); await loadTerms(sessionId);
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to create term"); }
    finally { setBusy(false); }
  }

  async function loadSections(classId: string) {
    try {
      const data = await api("/admin/classes/" + classId + "/sections");
      setSections(v => ({ ...v, [classId]: Array.isArray(data) ? data : data?.sections || [] }));
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to load sections"); }
  }

  async function createSection(classId: string, event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true);
    const form = new FormData(event.currentTarget);
    try {
      await api("/admin/classes/" + classId + "/sections", { method: "POST", body: JSON.stringify({ name: form.get("name") })});
      event.currentTarget.reset(); await loadSections(classId);
    } catch (e) { setError(e instanceof Error ? e.message : "Unable to create section"); }
    finally { setBusy(false); }
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="logo"><span>S</span><div><strong>Stonez</strong><small>School OS</small></div></div>
        <nav>
          <Link href="/dashboard">Overview</Link><Link className="active" href="/dashboard/academic">Academic</Link>
          <Link href="/dashboard/students">Students</Link><Link href="/dashboard/enrollments">Enrollment</Link><Link href="/dashboard/teacher-assignments">Teacher Assignments</Link>
          <Link href="/dashboard/attendance">Attendance</Link><Link href="/dashboard/assessments">Assessments</Link>
          <Link href="/dashboard/results">Results</Link><Link href="/dashboard/timetable">Timetable</Link>
          <Link href="/dashboard/operations">Operations</Link><Link href="/dashboard/users">Users & Roles</Link><Link href="/dashboard/audit">Audit Logs</Link>
        </nav>
      </aside>
      <main className="content">
        <header className="topbar"><div><p className="eyebrow">ACADEMIC MANAGEMENT</p><h1>Academic structure</h1><p className="muted">Manage sessions, terms, classes, sections and subjects for your school.</p></div></header>
        {error && <div className="error banner">{error}</div>}

        <section className="panel">
          <div className="panel-head"><div><h2>Academic sessions & terms</h2><p>Define the school calendar and term boundaries.</p></div></div>
          <form className="form-grid" onSubmit={createSession}>
            <input name="name" placeholder="2026/2027" required maxLength={100}/><input name="start_date" type="date" required/><input name="end_date" type="date" required/>
            <select name="status" defaultValue="planned"><option value="planned">Planned</option><option value="active">Active</option><option value="closed">Closed</option><option value="archived">Archived</option></select>
            <button disabled={busy}>Add session</button>
          </form>
          <div className="table-wrap"><table><thead><tr><th>Session</th><th>Dates</th><th>Status</th><th>Terms</th></tr></thead><tbody>
            {sessions.map(s => <tr key={s.id}><td><strong>{s.name}</strong></td><td>{dateValue(s.start_date)} → {dateValue(s.end_date)}</td><td><span className={"pill "+s.status}>{s.status}</span></td><td><button className="ghost" onClick={() => loadTerms(s.id)}>View terms</button>{terms[s.id] && <div className="nested-list">{terms[s.id].map(t => <div key={t.id}><strong>{t.name}</strong> · {dateValue(t.start_date)} → {dateValue(t.end_date)} · {t.status}</div>)}<form onSubmit={e => createTerm(s.id, e)} className="inline-form"><input name="name" required placeholder="e.g. First Term" /><input name="start_date" type="date" required/><input name="end_date" type="date" required/><select name="status" defaultValue="planned"><option>planned</option><option>active</option><option>closed</option></select><button disabled={busy}>Add term</button></form></div>}</td></tr>)}
          </tbody></table>{!sessions.length && <div className="empty">No academic sessions yet.</div>}</div>
        </section>

        <section className="grid-2">
          <div className="panel"><div className="panel-head"><div><h2>Classes & sections</h2><p>Set up the school's class structure.</p></div></div>
            <form className="form-grid" onSubmit={createClass}><input name="name" placeholder="JSS 1" required/><input name="level" type="number" min="0" placeholder="Level"/><button disabled={busy}>Add class</button></form>
            <div className="table-wrap"><table><thead><tr><th>Class</th><th>Level</th><th>Sections</th></tr></thead><tbody>{classes.map(c=><tr key={c.id}><td><strong>{c.name}</strong></td><td>{c.level}</td><td><button className="ghost" onClick={() => loadSections(c.id)}>View</button>{sections[c.id]?.map(s=><div key={s.id} className="nested-item">{s.name}</div>)}{sections[c.id] && <form className="inline-form" onSubmit={e => createSection(c.id,e)}><input name="name" placeholder="A" required/><button disabled={busy}>Add</button></form>}</td></tr>)}</tbody></table>{!classes.length && <div className="empty">No classes yet.</div>}</div>
          </div>
          <div className="panel"><div className="panel-head"><div><h2>Subjects</h2><p>Maintain the school's subject catalogue.</p></div></div>
            <form className="form-grid" onSubmit={createSubject}><input name="code" placeholder="MTH" required/><input name="name" placeholder="Mathematics" required/><input name="description" placeholder="Description"/><button disabled={busy}>Add subject</button></form>
            <div className="table-wrap"><table><thead><tr><th>Code</th><th>Name</th><th>Status</th></tr></thead><tbody>{subjects.map(s=><tr key={s.id}><td><strong>{s.code}</strong></td><td>{s.name}</td><td>{s.active ? "Active" : "Inactive"}</td></tr>)}</tbody></table>{!subjects.length && <div className="empty">No subjects yet.</div>}</div>
          </div>
        </section>
      </main>
    </div>
  );
}
