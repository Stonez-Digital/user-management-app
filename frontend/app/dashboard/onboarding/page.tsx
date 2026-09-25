"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import Link from "next/link";

type Job = {
  id: string;
  kind: string;
  file_name: string;
  status: string;
  total: number;
  valid: number;
  created: number;
  updated: number;
  skipped: number;
  failed: number;
  send_credentials: boolean;
  credentials_sent: number;
  credential_emails_failed: number;
  created_at: string;
  updated_at: string;
};

type ErrorRow = { row: number; field?: string; message: string };
type Preview = { job: Job; preview: Record<string, string>[]; errors: ErrorRow[] };
type Student = {
  id: string;
  admission_number: string;
  user?: { name: string; email: string };
};

async function api(path: string, options: RequestInit = {}) {
  const token = localStorage.getItem("access_token");
  const r = await fetch("/backend" + path, {
    ...options,
    headers: { Authorization: "Bearer " + token, ...(options.headers || {}) },
  });
  const text = await r.text();
  let d: any = {};
  try { d = text ? JSON.parse(text) : {}; } catch { d = { error: text }; }
  if (r.status === 401) throw Error("Session expired");
  if (!r.ok) throw Error(d?.error?.message || d?.error || "Request failed");
  return d;
}

const templates: Record<string, string> = {
  students:
    "admission_number,first_name,last_name,other_names,date_of_birth,gender,email,phone,class,section,session,student_status,parent_identifier\nBED-001,Jane,Doe,,,female,jane@example.com,08000000000,JSS1,A,2026/2027,active,PARENT-001\n",
  teachers:
    "staff_id,first_name,last_name,other_names,email,phone,gender,employment_date,department,designation,subjects,classes,status\nT-001,John,Doe,,john@example.com,08000000000,male,2026-09-01,Science,Teacher,Mathematics,JSS1,active\n",
  parents:
    "parent_identifier,first_name,last_name,other_names,relationship,phone,email,address,occupation,student_admission_number,primary\nPARENT-001,Jane,Doe,,mother,08000000000,parent@example.com,,Trader,BED-001,true\n",
};

function downloadTemplate(kind: string) {
  const blob = new Blob([templates[kind]], { type: "text/csv;charset=utf-8" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = kind + "-import-template.csv";
  a.click();
  URL.revokeObjectURL(a.href);
}

export default function OnboardingPage() {
  const [kind, setKind] = useState("students");
  const [preview, setPreview] = useState<Preview | null>(null);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [students, setStudents] = useState<Student[]>([]);
  const [busy, setBusy] = useState(false);
  const [sendCredentials, setSendCredentials] = useState(true);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);

  const [person, setPerson] = useState({
    name: "",
    email: "",
    password: "",
    role: "student",
    admission_number: "",
    enrollment_status: "active",
    student_id: "",
    staff_id: "",
    phone: "",
    gender: "",
    department: "",
    designation: "",
    subjects: "",
    classes: "",
    parent_identifier: "",
    address: "",
    occupation: "",
    relationship: "parent",
    primary: true,
  });

  async function loadJobs() {
    try {
      const d = await api("/admin/onboarding/bulk");
      setJobs(Array.isArray(d?.imports) ? d.imports : []);
    } catch {}
  }

  async function loadStudents() {
    try {
      const d = await api("/admin/students");
      const list = Array.isArray(d) ? d : d?.students || [];
      setStudents(list);
    } catch {}
  }

  useEffect(() => {
    loadJobs();
    loadStudents();
  }, []);

  async function createPerson(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    setMessage("");
    try {
      const payload: Record<string, unknown> = {
        name: person.name,
        email: person.email,
        password: person.password,
        role: person.role,
      };
      if (person.role === "teacher") {
        payload.staff_id = person.staff_id;
        payload.phone = person.phone;
        payload.gender = person.gender;
        payload.department = person.department;
        payload.designation = person.designation;
        payload.subjects = person.subjects;
        payload.classes = person.classes;
      }
      if (person.role === "parent") {
        payload.parent_identifier = person.parent_identifier;
        payload.phone = person.phone;
        payload.address = person.address;
        payload.occupation = person.occupation;
      }
      if (person.role === "student") {
        payload.admission_number = person.admission_number;
        payload.enrollment_status = person.enrollment_status;
      }
      if (person.role === "parent") {
        if (person.student_id) payload.student_id = person.student_id;
        payload.relationship = person.relationship;
        payload.primary = person.primary;
      }
      const d = await api("/admin/onboarding/people", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      setMessage(
        `${person.role[0].toUpperCase() + person.role.slice(1)} account created successfully for ${d?.user?.name || person.name}.`,
      );
      setPerson({
        name: "",
        email: "",
        password: "",
        role: person.role,
        admission_number: "",
        staff_id: "",
        phone: "",
        gender: "",
        department: "",
        designation: "",
        subjects: "",
        classes: "",
        parent_identifier: "",
        address: "",
        occupation: "",
        enrollment_status: "active",
        student_id: "",
        relationship: "parent",
        primary: true,
      });
      await loadStudents();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to create account");
    } finally {
      setBusy(false);
    }
  }

  async function previewFile(file: File) {
    setBusy(true);
    setError("");
    setMessage("");
    setPreview(null);
    try {
      const fd = new FormData();
      fd.append("kind", kind);
      fd.append("send_credentials", String(sendCredentials));
      fd.append("file", file);
      const d = await api("/admin/onboarding/bulk/preview", { method: "POST", body: fd });
      setPreview(d);
      setMessage("Validation completed. Review the records before starting the import.");
      await loadJobs();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to validate import");
    } finally {
      setBusy(false);
    }
  }

  async function start() {
    if (!preview) return;
    setBusy(true);
    setError("");
    try {
      await api("/admin/onboarding/bulk/" + preview.job.id + "/start", { method: "POST" });
      setMessage("Import queued. You can leave this page; processing continues in the background.");
      await loadJobs();
      setPreview(null);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to start import");
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    const running = jobs.some((j) => ["queued", "running"].includes(j.status));
    if (!running) return;
    const t = setInterval(loadJobs, 2000);
    return () => clearInterval(t);
  }, [jobs]);

  const roleLabel = person.role === "student" ? "Student" : person.role === "teacher" ? "Teacher" : "Parent / Guardian";

  return (
    <div className="content standalone">
      <header className="topbar">
        <div>
          <Link className="back" href="/dashboard">← Dashboard</Link>
          <p className="eyebrow">SCHOOL ONBOARDING</p>
          <h1>Onboard people</h1>
          <p className="muted">Create one account at a time or onboard many people from a spreadsheet. Both workflows use the same school-scoped onboarding controls.</p>
        </div>
      </header>

      {error && <div className="error banner">{error}</div>}
      {message && <div className="panel"><strong>{message}</strong></div>}

      <section className="grid-2">
        <div className="panel">
          <div className="panel-head">
            <div>
              <p className="eyebrow">OPTION A</p>
              <h2>One-by-one onboarding</h2>
              <p>Create a single student, teacher, or parent account immediately.</p>
            </div>
            <span className="pill active">Single</span>
          </div>

          <form onSubmit={createPerson}>
            <div className="form-grid">
              <label>
                Person type
                <select value={person.role} onChange={(e) => setPerson({ ...person, role: e.target.value })}>
                  <option value="student">Student</option>
                  <option value="teacher">Teacher</option>
                  <option value="parent">Parent / Guardian</option>
                </select>
              </label>

              <label>
                Full name
                <input required minLength={2} maxLength={100} value={person.name} onChange={(e) => setPerson({ ...person, name: e.target.value })} placeholder="Full name" />
              </label>

              <label>
                Email
                <input required type="email" maxLength={255} value={person.email} onChange={(e) => setPerson({ ...person, email: e.target.value })} placeholder="person@example.com" />
              </label>

              <label>
                Temporary password
                <input required minLength={8} maxLength={128} type="password" value={person.password} onChange={(e) => setPerson({ ...person, password: e.target.value })} placeholder="Minimum 8 characters" />
              </label>

              {person.role === "teacher" && <>
                <label>Staff ID<input required minLength={2} maxLength={80} value={person.staff_id} onChange={(e) => setPerson({ ...person, staff_id: e.target.value })} placeholder="e.g. T-001" /></label>
                <label>Phone<input maxLength={30} value={person.phone} onChange={(e) => setPerson({ ...person, phone: e.target.value })} placeholder="080..." /></label>
                <label>Gender<input maxLength={30} value={person.gender} onChange={(e) => setPerson({ ...person, gender: e.target.value })} placeholder="e.g. female" /></label>
                <label>Department<input maxLength={100} value={person.department} onChange={(e) => setPerson({ ...person, department: e.target.value })} placeholder="e.g. Science" /></label>
                <label>Designation<input maxLength={100} value={person.designation} onChange={(e) => setPerson({ ...person, designation: e.target.value })} placeholder="e.g. Teacher" /></label>
                <label>Subjects<input maxLength={500} value={person.subjects} onChange={(e) => setPerson({ ...person, subjects: e.target.value })} placeholder="e.g. Mathematics" /></label>
                <label>Classes<input maxLength={500} value={person.classes} onChange={(e) => setPerson({ ...person, classes: e.target.value })} placeholder="e.g. JSS1" /></label>
              </>}

              {person.role === "student" && <>
                <label>
                  Admission number
                  <input required minLength={2} maxLength={50} value={person.admission_number} onChange={(e) => setPerson({ ...person, admission_number: e.target.value })} placeholder="e.g. BED-001" />
                </label>
                <label>
                  Enrollment status
                  <select value={person.enrollment_status} onChange={(e) => setPerson({ ...person, enrollment_status: e.target.value })}>
                    <option value="active">Active</option>
                    <option value="inactive">Inactive</option>
                    <option value="graduated">Graduated</option>
                    <option value="withdrawn">Withdrawn</option>
                  </select>
                </label>
              </>}

              {person.role === "parent" && <>
                <label>Parent identifier<input maxLength={100} value={person.parent_identifier} onChange={(e) => setPerson({ ...person, parent_identifier: e.target.value })} placeholder="e.g. PARENT-001" /></label>
                <label>Phone<input maxLength={30} value={person.phone} onChange={(e) => setPerson({ ...person, phone: e.target.value })} placeholder="080..." /></label>
                <label>Address<input maxLength={255} value={person.address} onChange={(e) => setPerson({ ...person, address: e.target.value })} placeholder="Home address" /></label>
                <label>Occupation<input maxLength={100} value={person.occupation} onChange={(e) => setPerson({ ...person, occupation: e.target.value })} placeholder="Occupation" /></label>
                <label>
                  Link to student
                  <select value={person.student_id} onChange={(e) => setPerson({ ...person, student_id: e.target.value })}>
                    <option value="">Do not link yet</option>
                    {students.map((s) => <option key={s.id} value={s.id}>{s.admission_number} — {s.user?.name || "Student"}</option>)}
                  </select>
                </label>
                <label>
                  Relationship
                  <input maxLength={40} value={person.relationship} onChange={(e) => setPerson({ ...person, relationship: e.target.value })} placeholder="e.g. mother" />
                </label>
                <label className="checkbox-row">
                  <input type="checkbox" checked={person.primary} onChange={(e) => setPerson({ ...person, primary: e.target.checked })} />
                  Primary guardian for the selected student
                </label>
              </>}
            </div>

            <div className="panel-head">
              <p className="muted">Only school-level roles can be created here. Platform and school administrator accounts remain protected.</p>
              <button disabled={busy}>{busy ? "Creating…" : `Create ${roleLabel}`}</button>
            </div>
          </form>
        </div>

        <div className="panel">
          <div className="panel-head">
            <div>
              <p className="eyebrow">OPTION B</p>
              <h2>Bulk onboarding</h2>
              <p>Import hundreds or thousands of students, teachers, or parents with CSV/XLSX.</p>
            </div>
            <span className="pill active">Bulk</span>
          </div>

          <div className="actions">
            {(["students", "teachers", "parents"] as const).map((k) => (
              <button key={k} className={kind === k ? "" : "ghost"} onClick={() => { setKind(k); setPreview(null); }}>
                {k[0].toUpperCase() + k.slice(1)}
              </button>
            ))}
          </div>

          <div className="form-grid">
            <div>
              <p><strong>{kind === "students" ? "Students" : kind === "teachers" ? "Teachers" : "Parents / Guardians"} template</strong></p>
              <button className="ghost" onClick={() => downloadTemplate(kind)}>Download CSV template</button>
            </div>
            <label className="checkbox-row">
              <input type="checkbox" checked={sendCredentials} onChange={(e) => setSendCredentials(e.target.checked)} />
              Email temporary login credentials to newly created accounts
            </label>
            <label>
              Upload CSV or XLSX
              <input ref={fileRef} type="file" accept=".csv,.xlsx" onChange={(e) => e.target.files?.[0] && previewFile(e.target.files[0])} />
            </label>
          </div>

          <div className="role-list">
            <div><span>Students</span><strong>Admission number + account</strong></div>
            <div><span>Teachers</span><strong>Staff ID + account</strong></div>
            <div><span>Parents</span><strong>Parent identifier + guardian links</strong></div>
          </div>
        </div>
      </section>

      {preview && <section className="panel">
        <div className="panel-head">
          <div><h2>Bulk validation</h2><p>{preview.job.total.toLocaleString()} records detected. No database import has started.</p></div>
          <span className={"pill " + (preview.errors.length ? "suspended" : "active")}>{preview.errors.length ? preview.errors.length + " errors" : "Ready"}</span>
        </div>
        <div className="stats">
          <Stat label="Total" value={preview.job.total} detail="Uploaded rows" />
          <Stat label="Valid" value={preview.job.valid} detail="Ready to import" />
          <Stat label="Errors" value={preview.errors.length} detail="Rows needing correction" />
        </div>
        {preview.errors.length > 0 && <div className="table-wrap"><table><thead><tr><th>Row</th><th>Field</th><th>Problem</th></tr></thead><tbody>{preview.errors.slice(0, 50).map((e, i) => <tr key={i}><td>{e.row}</td><td>{e.field || "—"}</td><td>{e.message}</td></tr>)}</tbody></table></div>}
        <div className="panel-head">
          <div><h3>Preview</h3><p>First 10 rows only.</p></div>
          <button disabled={busy || preview.job.valid === 0} onClick={start}>{busy ? "Starting…" : "Start bulk import"}</button>
        </div>
      </section>}

      <section className="panel">
        <div className="panel-head">
          <div><h2>Bulk import history</h2><p>Progress and results are school-scoped.</p></div>
          <button className="ghost" onClick={loadJobs}>Refresh</button>
        </div>
        <div className="table-wrap">
          <table><thead><tr><th>Type</th><th>File</th><th>Total</th><th>Created</th><th>Skipped</th><th>Failed</th><th>Credentials</th><th>Status</th></tr></thead>
          <tbody>{jobs.map((j) => <tr key={j.id}><td>{j.kind}</td><td>{j.file_name}</td><td>{j.total.toLocaleString()}</td><td>{j.created.toLocaleString()}</td><td>{j.skipped.toLocaleString()}</td><td>{j.failed.toLocaleString()}</td><td>{j.send_credentials ? `${j.credentials_sent.toLocaleString()} sent${j.credential_emails_failed ? ` / ${j.credential_emails_failed} failed` : ""}` : "Off"}</td><td><span className={"pill " + (j.status.includes("completed") ? "active" : "")}>{j.status}</span></td></tr>)}</tbody>
          </table>
          {!jobs.length && <div className="empty">No bulk imports yet.</div>}
        </div>
      </section>

      <section className="panel">
        <h2>Which method should I use?</h2>
        <div className="role-list">
          <div><span>One-by-one</span><strong>New admission, replacement staff, or individual parent</strong></div>
          <div><span>Bulk</span><strong>Existing school records or large migrations</strong></div>
          <div><span>Both</span><strong>Use them side by side without changing the school data model</strong></div>
        </div>
      </section>
    </div>
  );
}

function Stat({ label, value, detail }: { label: string; value: number; detail: string }) {
  return <div className="panel"><span className="muted">{label}</span><h2>{value.toLocaleString()}</h2><small>{detail}</small></div>;
}
