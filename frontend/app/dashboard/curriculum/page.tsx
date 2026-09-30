"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

type Session = { id: string; name: string; status: string };
type SchoolClass = { id: string; name: string; level: number };
type Subject = { id: string; code: string; name: string; active: boolean };
type CurriculumSubject = { id: string; code: string; name: string; category: string; required: boolean; selection_group?: string | null; active: boolean };
type CurriculumClass = { id: string; code: string; name: string; subjects: CurriculumSubject[] };
type CurriculumLevel = { id: string; code: string; name: string; display_order: number; classes: CurriculumClass[] };
type Curriculum = { id: string; code: string; name: string; authority: string; version: string; description?: string; status: string; levels: CurriculumLevel[] };

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

export default function CurriculumPage() {
  const router = useRouter();
  const [curricula, setCurricula] = useState<Curriculum[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [classes, setClasses] = useState<SchoolClass[]>([]);
  const [subjects, setSubjects] = useState<Subject[]>([]);
  const [curriculumId, setCurriculumId] = useState("");
  const [sessionId, setSessionId] = useState("");
  const [selectedClassId, setSelectedClassId] = useState("");
  const [schoolClassId, setSchoolClassId] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const curriculum = curricula.find(v => v.id === curriculumId);
  const selectedClass = curriculum?.levels.flatMap(v => v.classes).find(v => v.id === selectedClassId);

  async function load() {
    try {
      const [catalogue, s, c, sub] = await Promise.all([
        api("/admin/national-curriculum"),
        api("/admin/academic-sessions"),
        api("/admin/classes"),
        api("/admin/subjects"),
      ]);
      const nextCurricula = Array.isArray(catalogue) ? catalogue : catalogue?.curricula || [];
      const nextSessions = Array.isArray(s) ? s : s?.sessions || [];
      setCurricula(nextCurricula);
      setSessions(nextSessions);
      setClasses(Array.isArray(c) ? c : c?.classes || []);
      setSubjects(Array.isArray(sub) ? sub : sub?.subjects || []);
      if (!curriculumId && nextCurricula[0]) setCurriculumId(nextCurricula[0].id);
      if (!sessionId) {
        const active = nextSessions.find((v: Session) => v.status === "active") || nextSessions[0];
        if (active) setSessionId(active.id);
      }
      setError("");
    } catch (e) {
      const text = e instanceof Error ? e.message : "Unable to load curriculum";
      setError(text);
      if (text === "Session expired") router.push("/");
    }
  }

  useEffect(() => { load(); }, [router]);

  const classOptions = useMemo(() => curriculum?.levels.flatMap(level => level.classes.map(cls => ({ ...cls, levelName: level.name }))) || [], [curriculum]);

  async function adopt() {
    if (!selectedClass || !schoolClassId || !sessionId || !curriculum) return;
    setBusy(true); setMessage(""); setError("");
    let createdSubjects = 0;
    let mapped = 0;
    try {
      const currentSubjects = [...subjects];
      for (const item of selectedClass.subjects.filter(v => v.active)) {
        let subject = currentSubjects.find(v => v.code.toUpperCase() === item.code.toUpperCase());
        if (!subject) {
          const createdSubject = await api("/admin/subjects", {
            method: "POST",
            body: JSON.stringify({ code: item.code, name: item.name, description: "National curriculum subject" }),
          }) as Subject;
          subject = createdSubject;
          currentSubjects.push(createdSubject);
          createdSubjects++;
        }
        if (!subject) throw new Error("Unable to resolve school subject");

        try {
          await api("/admin/class-subjects", {
            method: "POST",
            body: JSON.stringify({
              academic_session_id: sessionId,
              class_id: schoolClassId,
              subject_id: subject.id,
              curriculum_version: curriculum.version,
              category: item.category,
              required: item.required,
              selection_group: item.selection_group || null,
              active: true,
            }),
          });
          mapped++;
        } catch (e) {
          const text = e instanceof Error ? e.message : "";
          if (!text.toLowerCase().includes("already") && !text.toLowerCase().includes("duplicate")) throw e;
        }
      }
      setSubjects(currentSubjects);
      setMessage(`Applied ${curriculum.name} · ${selectedClass.name}: ${mapped} subject mappings ready. ${createdSubjects} school subjects were created.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to apply curriculum");
    } finally { setBusy(false); }
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="logo"><span>S</span><div><strong>Stonez</strong><small>School OS</small></div></div>
        <nav>
          <Link href="/dashboard">Overview</Link>
          <Link className="active" href="/dashboard/curriculum">Curriculum</Link>
          <Link href="/dashboard/academic">Academic</Link>
          <Link href="/dashboard/students">Students</Link><Link href="/dashboard/enrollments">Enrollment</Link>
          <Link href="/dashboard/teacher-assignments">Teacher Assignments</Link><Link href="/dashboard/attendance">Attendance</Link>
          <Link href="/dashboard/assessments">Assessments</Link><Link href="/dashboard/results">Results</Link>
          <Link href="/dashboard/timetable">Timetable</Link><Link href="/dashboard/operations">Operations</Link>
          <Link href="/dashboard/users">Users & Roles</Link><Link href="/dashboard/audit">Audit Logs</Link>
        </nav>
      </aside>
      <main className="content">
        <header className="topbar">
          <div><p className="eyebrow">CURRICULUM</p><h1>National Curriculum</h1><p className="muted">Browse the platform's national catalogue and map it to your school's classes without duplicating curriculum data.</p></div>
        </header>

        {error && <div className="error banner">{error}</div>}
        {message && <div className="success banner">{message}</div>}

        <section className="panel">
          <div className="panel-head"><div><h2>Curriculum catalogue</h2><p>National curriculum data is platform-level. Your school adopts the subjects it needs for each class and academic session.</p></div></div>
          <div className="form-grid">
            <label>Curriculum<select value={curriculumId} onChange={e => { setCurriculumId(e.target.value); setSelectedClassId(""); }}>
              {curricula.map(v => <option key={v.id} value={v.id}>{v.name} · {v.version}</option>)}
            </select></label>
            <label>Academic session<select value={sessionId} onChange={e => setSessionId(e.target.value)}>
              <option value="">Select session</option>{sessions.map(v => <option key={v.id} value={v.id}>{v.name} · {v.status}</option>)}
            </select></label>
          </div>
          {curriculum && <div className="nested-list"><strong>{curriculum.authority}</strong> · {curriculum.version} · {curriculum.status}<div className="muted">{curriculum.description}</div></div>}
        </section>

        <section className="grid-2">
          <div className="panel">
            <div className="panel-head"><div><h2>National classes</h2><p>Select a class to inspect its official catalogue subjects.</p></div></div>
            <div className="table-wrap"><table><thead><tr><th>Level</th><th>Class</th><th>Subjects</th></tr></thead><tbody>
              {classOptions.map(v => <tr key={v.id} onClick={() => setSelectedClassId(v.id)} style={{ cursor: "pointer" }}><td>{v.levelName}</td><td><strong>{v.name}</strong></td><td>{v.subjects.length}</td></tr>)}
            </tbody></table></div>
          </div>

          <div className="panel">
            <div className="panel-head"><div><h2>{selectedClass ? selectedClass.name : "Select a class"}</h2><p>{selectedClass ? "Subjects in the national catalogue for this class." : "Choose a national class on the left."}</p></div></div>
            {selectedClass && <div className="table-wrap"><table><thead><tr><th>Code</th><th>Subject</th><th>Category</th><th>Required</th></tr></thead><tbody>
              {selectedClass.subjects.map(v => <tr key={v.id}><td><strong>{v.code}</strong></td><td>{v.name}</td><td>{v.category}</td><td>{v.required ? "Yes" : "Optional"}</td></tr>)}
            </tbody></table></div>}
          </div>
        </section>

        <section className="panel">
          <div className="panel-head"><div><h2>Adopt for a school class</h2><p>This creates or reuses the school's subject records and maps them to the selected class for the selected session. It does not modify the national catalogue.</p></div></div>
          <div className="form-grid">
            <label>National class<select value={selectedClassId} onChange={e => setSelectedClassId(e.target.value)} disabled={!curriculum}>
              <option value="">Select national class</option>{classOptions.map(v => <option key={v.id} value={v.id}>{v.levelName} · {v.name}</option>)}
            </select></label>
            <label>School class<select value={schoolClassId} onChange={e => setSchoolClassId(e.target.value)}>
              <option value="">Select school class</option>{classes.map(v => <option key={v.id} value={v.id}>{v.name}</option>)}
            </select></label>
            <button disabled={busy || !selectedClass || !schoolClassId || !sessionId} onClick={adopt}>{busy ? "Applying…" : "Adopt curriculum subjects"}</button>
          </div>
        </section>
      </main>
    </div>
  );
}
