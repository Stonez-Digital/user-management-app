"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { AcademicContextProvider, useAcademicContext } from "../../lib/academic-context";
import AcademicSelector from "../../components/academic-selector";
import { normalizeAuthUser, type AuthUser } from "../../lib/auth-session";

type DashboardData = {
  school: {
    id: string;
    name: string;
    logo_url?: string;
    address?: string;
    contact_email?: string;
    contact_phone?: string;
    administrator: { id: string; name: string; email: string };
  };
  academic_context: { session_id?: string; session_name?: string; term_id?: string; term_name?: string };
  overview: {
    students: number; active_enrollments: number; teachers: number; parents: number;
    classes: number; sections: number; subjects: number;
  };
  attendance: {
    expected_today: number; present: number; absent: number; late: number; excused: number;
    percentage: number; recorded: boolean;
  };
  finance: {
    total_invoiced: number; amount_paid: number; outstanding_balance: number; outstanding_invoices: number;
    recent_invoices: { id: string; invoice_number: string; total_amount: number; paid_amount: number; balance: number; status: string; created_at: string }[];
    recent_payments: { id: string; invoice_id: string; invoice_number: string; amount: number; provider: string; reference: string; status: string; paid_at?: string; created_at: string }[];
  };
  alerts: { key: string; severity: string; count: number; message: string; action: string }[];
  recent_activity: { id: string; action: string; resource: string; resource_id?: string; actor_id?: string; actor_name?: string; created_at: string }[];
  health: Record<string, string>;
};

type PlatformData = {
  schools?: { id: string; name: string; code: string; status: string; user_count: number }[];
  monitoring?: { database: { status: string }; schools: { total: number; pending: number; active: number; suspended: number }; users: number; activity: { action: string; resource: string; created_at: string }[] };
};

async function api(path: string) {
  const token = localStorage.getItem("access_token");
  const response = await fetch("/backend" + path, { headers: { Authorization: "Bearer " + token } });
  const data = await response.json().catch(() => ({}));
  if (response.status === 401) throw new Error("Session expired");
  if (!response.ok) throw new Error(data?.error?.message || data?.error || "Request failed");
  return data;
}

export default function Dashboard() {
  const router = useRouter();
  const [me, setMe] = useState<AuthUser | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api("/me").then(payload => {
      const user = normalizeAuthUser(payload);
      if (!user) throw new Error("Unable to load your account profile");
      setMe(user);
      if (user.role === "teacher") router.replace("/dashboard/teacher");
      else if (user.role === "student") router.replace("/dashboard/student");
      else if (user.role === "parent") router.replace("/dashboard/parent");
      else if (user.role === "accountant" || user.role === "staff") router.replace("/dashboard/operations");
    }).catch(e => setError(e instanceof Error ? e.message : "Unable to load your account"))
      .finally(() => setLoading(false));
  }, [router]);

  if (loading) return <main className="content"><div className="panel"><strong>Loading your workspace…</strong><p className="muted">Verifying your account and school access.</p></div></main>;
  if (error) return <main className="content"><div className="error banner">{error}</div></main>;
  if (me?.role === "school_admin") return <AcademicContextProvider><AcademicSelector /><SchoolAdminDashboard me={me} /></AcademicContextProvider>;
  if (me?.role === "super_admin") return <SuperAdminDashboard />;
  return <main className="content"><div className="panel"><h2>Workspace</h2><p className="muted">Your role uses a separate operational workspace.</p><Link href="/dashboard/operations">Open workspace →</Link></div></main>;
}

function SchoolAdminDashboard({ me }: { me: AuthUser }) {
  const router = useRouter();
  const { sessionId, termId } = useAcademicContext();
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  async function load() {
    try {
      setError("");
      setLoading(true);
      const query = new URLSearchParams();
      if (sessionId) query.set("academic_session_id", sessionId);
      if (termId) query.set("term_id", termId);
      const value = await api("/admin/dashboard?" + query.toString());
      setData(value);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Unable to load school command center");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { if (sessionId || termId) load(); }, [sessionId, termId]);

  if (loading && !data) return <main className="content"><div className="panel"><strong>Loading school command center…</strong><p className="muted">Loading live school operations.</p></div></main>;
  if (error && !data) return <main className="content"><div className="error banner">{error}</div><button onClick={load}>Retry</button></main>;
  if (!data) return null;

  const money = (value: number) => new Intl.NumberFormat("en-NG", { style: "currency", currency: "NGN", maximumFractionDigits: 0 }).format(value);
  const health = [
    ["Academic setup", data.health.academic_setup],
    ["Enrollment", data.health.enrollment],
    ["Teacher allocation", data.health.teacher_allocation],
    ["Attendance activity", data.health.attendance_activity],
    ["Assessment activity", data.health.assessment_activity],
    ["Finance activity", data.health.finance_activity],
  ];
  const quickActions = [
    ["Add Student", "/dashboard/onboarding", "Create a student account"],
    ["Add Teacher", "/dashboard/onboarding", "Create a teacher account"],
    ["Add Parent", "/dashboard/onboarding", "Create a parent account"],
    ["Bulk Import", "/dashboard/onboarding", "Import people in bulk"],
    ["Manage Enrollment", "/dashboard/enrollments", "Review student placement"],
    ["Manage Classes", "/dashboard/academic", "Manage classes and sections"],
    ["Manage Subjects", "/dashboard/academic", "Manage school subjects"],
    ["Assign Teachers", "/dashboard/teacher-assignments", "Allocate teaching assignments"],
    ["Record Attendance", "/dashboard/attendance", "Open daily attendance"],
    ["Create Assessment", "/dashboard/assessments", "Create an assessment"],
    ["View Results", "/dashboard/results", "Review results"],
    ["Create Invoice", "/dashboard/finance", "Create student billing"],
    ["View Payments", "/dashboard/finance", "Review payment records"],
  ];

  return <main className="content command-center">
    <header className="topbar">
      <div>
        <p className="eyebrow">SCHOOL OPERATIONS</p>
        <h1>{data.school.name}</h1>
        <p className="muted">Operational command center for {data.academic_context.session_name || "the selected session"}{data.academic_context.term_name ? " · " + data.academic_context.term_name : ""}.</p>
      </div>
      <div className="status"><span /> {me.name || data.school.administrator.name || "Administrator"}</div>
    </header>

    {error && <div className="error banner">{error}</div>}

    <section className="command-identity">
      <div className="command-school">
        {data.school.logo_url ? <img src={data.school.logo_url} alt={data.school.name + " logo"} /> : <div className="command-logo-placeholder">{data.school.name.charAt(0).toUpperCase()}</div>}
        <div>
          <strong>{data.school.name}</strong>
          <span>{data.school.address || "School address not configured"}</span>
          <span>{[data.school.contact_email, data.school.contact_phone].filter(Boolean).join(" · ") || "School contact information not configured"}</span>
        </div>
      </div>
      <div className="command-admin"><span>Administrator</span><strong>{data.school.administrator.name || me.name || "School administrator"}</strong><small>{data.school.administrator.email || me.email || "—"}</small></div>
    </section>

    <section className="command-context">
      <div><span>Current session</span><strong>{data.academic_context.session_name || "No active session"}</strong></div>
      <div><span>Current term</span><strong>{data.academic_context.term_name || "No active term"}</strong></div>
      <button className="ghost light" onClick={load}>Refresh</button>
    </section>

    <section className="stats">
      <Stat label="Students" value={data.overview.students} detail="Student profiles" />
      <Stat label="Active enrollments" value={data.overview.active_enrollments} detail="Selected session" />
      <Stat label="Teachers" value={data.overview.teachers} detail="Active teacher accounts" />
      <Stat label="Parents / guardians" value={data.overview.parents} detail="Active parent accounts" />
      <Stat label="Classes" value={data.overview.classes} detail="School classes" />
      <Stat label="Sections" value={data.overview.sections} detail="Configured sections" />
      <Stat label="Subjects" value={data.overview.subjects} detail="Active subjects" />
      <Stat label="Outstanding invoices" value={data.finance.outstanding_invoices} detail={money(data.finance.outstanding_balance)} />
    </section>

    <section className="grid-2">
      <div className="panel">
        <div className="panel-head"><div><h2>What needs attention?</h2><p>Actions are based on live school data.</p></div></div>
        {data.alerts.length ? <div className="alert-list">{data.alerts.map(a => <Link className={"command-alert " + a.severity} key={a.key} href={a.action}><div><strong>{a.count}</strong><span>{a.message}</span></div><b>Review →</b></Link>)}</div> : <div className="empty">No operational gaps detected for the selected context.</div>}
      </div>
      <div className="panel">
        <div className="panel-head"><div><h2>Attendance today</h2><p>{data.attendance.recorded ? "Attendance has been recorded." : "No attendance has been recorded today."}</p></div></div>
        <div className="attendance-hero"><strong>{data.attendance.percentage.toFixed(1)}%</strong><span>attendance</span></div>
        <div className="mini-metrics">
          <Metric label="Expected" value={data.attendance.expected_today} />
          <Metric label="Present" value={data.attendance.present} />
          <Metric label="Absent" value={data.attendance.absent} />
          <Metric label="Late" value={data.attendance.late} />
          <Metric label="Excused" value={data.attendance.excused} />
        </div>
        <Link href="/dashboard/attendance">Open attendance →</Link>
      </div>
    </section>

    <section className="panel">
      <div className="panel-head"><div><h2>Finance overview</h2><p>Selected-term invoice and payment activity.</p></div><Link href="/dashboard/finance">Open finance →</Link></div>
      <div className="finance-summary">
        <Metric label="Total invoiced" value={money(data.finance.total_invoiced)} />
        <Metric label="Amount paid" value={money(data.finance.amount_paid)} />
        <Metric label="Outstanding" value={money(data.finance.outstanding_balance)} />
        <Metric label="Outstanding invoices" value={data.finance.outstanding_invoices} />
      </div>
      <div className="grid-2 finance-tables">
        <Table title="Recent invoices" empty="No invoices have been created yet." rows={data.finance.recent_invoices.map(i => [i.invoice_number, money(i.total_amount), money(i.balance), i.status])} headers={["Invoice","Total","Balance","Status"]} />
        <Table title="Recent payments" empty="No payments have been recorded yet." rows={data.finance.recent_payments.map(p => [p.invoice_number || p.invoice_id.slice(0, 8), money(p.amount), p.provider, p.status])} headers={["Invoice","Amount","Provider","Status"]} />
      </div>
    </section>

    <section className="panel">
      <div className="panel-head"><div><h2>Quick actions</h2><p>Go directly to existing school workflows.</p></div></div>
      <div className="quick-actions">{quickActions.map(([label, href, description]) => <Link className="quick-action" key={label} href={href}><strong>{label}</strong><span>{description}</span><b>→</b></Link>)}</div>
    </section>

    <section className="grid-2">
      <div className="panel">
        <div className="panel-head"><div><h2>Operational health</h2><p>Descriptive status only — no artificial score.</p></div></div>
        <div className="health-list">{health.map(([label, status]) => <div key={label}><span>{label}</span><strong className={"health-" + status.toLowerCase().replaceAll(" ", "-")}>{status}</strong></div>)}</div>
      </div>
      <div className="panel">
        <div className="panel-head"><div><h2>Recent activity</h2><p>Latest school-scoped audit events.</p></div><Link href="/dashboard/audit">Open audit logs →</Link></div>
        <div className="activity-list">{data.recent_activity.map(a => <div className="activity-row" key={a.id}><div><strong>{a.action}</strong><span>{a.resource}{a.actor_name ? " · " + a.actor_name : ""}</span></div><time>{new Date(a.created_at).toLocaleString()}</time></div>)}</div>
        {!data.recent_activity.length && <div className="empty">No recent school activity.</div>}
      </div>
    </section>

    <div className="command-footer"><span>Signed in as {me.email || data.school.administrator.email || "school administrator"}</span><button className="ghost light" onClick={() => router.push("/dashboard/school")}>School settings</button></div>
  </main>;
}

function SuperAdminDashboard() {
  const [data, setData] = useState<PlatformData>({});
  const [error, setError] = useState("");
  useEffect(() => { Promise.all([api("/platform/schools"), api("/platform/monitoring")]).then(([schools, monitoring]) => setData({ schools: schools.schools || [], monitoring })).catch(e => setError(e.message)); }, []);
  const schools = data.schools || [];
  return <main className="content">
    <header className="topbar"><div><p className="eyebrow">STONEZ DIGITAL</p><h1>Platform operations</h1><p className="muted">Monitor onboarded school tenants.</p></div><div className="status"><span /> Platform operations</div></header>
    {error && <div className="error banner">{error}</div>}
    <section className="stats">
      <Stat label="Total schools" value={data.monitoring?.schools.total || schools.length} detail="Registered tenants" />
      <Stat label="Active schools" value={data.monitoring?.schools.active || 0} detail="Active tenants" />
      <Stat label="Pending review" value={data.monitoring?.schools.pending || 0} detail="Awaiting approval" />
      <Stat label="Users" value={data.monitoring?.users || 0} detail="Across tenants" />
    </section>
    <section className="panel"><div className="panel-head"><div><h2>Tenant monitoring</h2><p>School tenants remain isolated from one another.</p></div><Link href="/platform/schools">Manage schools →</Link></div><div className="table-wrap"><table><thead><tr><th>School</th><th>Code</th><th>Users</th><th>Status</th></tr></thead><tbody>{schools.map(s => <tr key={s.id}><td><strong>{s.name}</strong></td><td>{s.code}</td><td>{s.user_count}</td><td><span className={"pill " + s.status}>{s.status}</span></td></tr>)}</tbody></table>{!schools.length && <div className="empty">No school tenants onboarded yet.</div>}</div></section>
  </main>;
}

function Stat({ label, value, detail }: { label: string; value: number | string; detail: string }) {
  return <div className="stat"><span>{label}</span><strong>{value}</strong><small>{detail}</small></div>;
}
function Metric({ label, value }: { label: string; value: number | string }) {
  return <div className="metric"><span>{label}</span><strong>{value}</strong></div>;
}
function Table({ title, empty, rows, headers }: { title: string; empty: string; rows: string[][]; headers: string[] }) {
  return <div className="command-table"><h3>{title}</h3><div className="table-wrap"><table><thead><tr>{headers.map(h => <th key={h}>{h}</th>)}</tr></thead><tbody>{rows.map((r, i) => <tr key={i}>{r.map((v, j) => <td key={j}>{v}</td>)}</tr>)}</tbody></table>{!rows.length && <div className="empty">{empty}</div>}</div></div>;
}
