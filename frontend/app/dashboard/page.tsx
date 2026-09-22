"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
import {useRouter} from "next/navigation";
import { normalizeAuthUser } from "../../lib/auth-session";

type User={id:string;name:string;email:string;role:string;active:boolean;school_name?:string|null};
type School={id:string;name:string;code:string;status:string;user_count:number};
type Student={id:string;admission_number:string;gender:string;enrollment_status:string;user?:{name:string;email:string}};
type AcademicSession={id:string;name:string;status:string};
type ClassRecord={id:string;name:string;level:number};
type Subject={id:string;code:string;name:string;active:boolean};
type Monitoring={database:{status:string};schools:{total:number;pending:number;active:number;suspended:number};users:number;activity:{action:string;resource:string;created_at:string}[];checked_at:string};

async function api(path:string){
 const t=localStorage.getItem("access_token");
 const r=await fetch("/backend"+path,{headers:{Authorization:"Bearer "+t}});
 if(r.status===401)throw Error("Session expired");
 const d=await r.json();if(!r.ok)throw Error(d?.error?.message||d?.error||"Request failed");return d;
}

export default function Dashboard(){
 const router=useRouter();
 const[users,setUsers]=useState<User[]>([]);
 const[students,setStudents]=useState<Student[]>([]);
 const[schools,setSchools]=useState<School[]>([]);
 const[me,setMe]=useState<User|null>(null);
 const[school,setSchool]=useState<School|null>(null);
 const[monitoring,setMonitoring]=useState<Monitoring|null>(null);
 const[sessions,setSessions]=useState<AcademicSession[]>([]);
 const[classes,setClasses]=useState<ClassRecord[]>([]);
 const[subjects,setSubjects]=useState<Subject[]>([]);
 const[error,setError]=useState("");
 const[loading,setLoading]=useState(true);
 const[lastUpdated,setLastUpdated]=useState<Date|null>(null);

 useEffect(()=>{
  let timer:ReturnType<typeof setInterval>|undefined;
  api("/me").then(async payload=>{
   const m=normalizeAuthUser(payload);
   if(!m) throw Error("Unable to load your account profile");
   const sessionUser:User={id:m.id,name:m.name??"",email:m.email??"",role:m.role,active:m.active??true,school_name:m.school_name??null};
   setMe(sessionUser);
   if(m.role==="teacher"){router.replace("/dashboard/teacher");return}
   if(m.role==="super_admin"){
    const load=async()=>{try{const [d,mn]=await Promise.all([api("/platform/schools"),api("/platform/monitoring")]);setSchools(Array.isArray(d?.schools)?d.schools:[]);setMonitoring(mn);setLastUpdated(new Date());setError("")}catch(e){setError(e instanceof Error?e.message:"Unable to refresh platform monitoring")}};
    await load();
    timer=setInterval(load,30000);
    return;
   }
   const s=await api("/admin/school");
   setSchool(s);
   const [u,st,ac,cl,su]=await Promise.all([api("/admin/users"),api("/admin/students"),api("/admin/academic-sessions"),api("/admin/classes"),api("/admin/subjects")]);
   setUsers(Array.isArray(u)?u:u?.users||[]);
   setStudents(Array.isArray(st)?st:st?.students||[]);
   setSessions(Array.isArray(ac)?ac:ac?.sessions||[]);
   setClasses(Array.isArray(cl)?cl:cl?.classes||[]);
   setSubjects(Array.isArray(su)?su:su?.subjects||[]);
  }).catch(e=>{localStorage.removeItem("access_token");localStorage.removeItem("refresh_token");setError(e instanceof Error?e.message:"Unable to load your account");router.push("/")}).finally(()=>setLoading(false));
  return()=>{if(timer)clearInterval(timer)};
 },[router]);

 function logout(){localStorage.clear();router.push("/")}

 const isPlatform=me?.role==="super_admin";
 const activeSchools=schools.filter(s=>s.status==="active").length;
 const suspendedSchools=schools.filter(s=>s.status==="suspended").length;
 const platformUsers=schools.reduce((sum,s)=>sum+s.user_count,0);

 return <div className="app-shell">
  <aside className="sidebar">
   <div className="logo"><span>S</span><div><strong>Stonez</strong><small>School OS</small></div></div>
   <nav>
    <Link className="active" href="/dashboard">Overview</Link>
    {isPlatform&&<Link href="/platform/schools">Platform Schools</Link>}
    {!isPlatform&&<><Link href="/dashboard/school">School Setup</Link><Link href="/dashboard/academic">Academic</Link><Link href="/dashboard/students">Students</Link><Link href="/dashboard/onboarding">Onboarding</Link><Link href="/dashboard/enrollments">Enrollment</Link><Link href="/dashboard/teacher-assignments">Teacher Assignments</Link><Link href="/dashboard/operations">Operations</Link><Link href="/dashboard/attendance">Attendance</Link><Link href="/dashboard/assessments">Assessments</Link><Link href="/dashboard/results">Results</Link><Link href="/dashboard/notifications">Notifications</Link><Link href="/dashboard/users">Users & Roles</Link><Link href="/dashboard/audit">Audit Logs</Link></>}
   </nav>
   <div className="sidebar-bottom"><div className="mini-user"><div className="avatar">{me?.name?.[0]||"A"}</div><div><strong>{me?.name||"Administrator"}</strong><small>{isPlatform?"Stonez Digital Platform":me?.role||"school_admin"}</small></div></div><button className="ghost" onClick={logout}>Sign out</button></div>
  </aside>
  <main className="content">
   {loading&&<div className="panel"><strong>Loading your workspace…</strong><p className="muted">Verifying your Stonez Digital account and school access.</p></div>}
   {!loading&&<header className="topbar">
    <div><p className="eyebrow">{isPlatform?"STONEZ DIGITAL":"SCHOOL ADMINISTRATION"}</p><h1>{isPlatform?"Stonez Digital Platform":"School overview"}</h1><p className="muted">{isPlatform?"Monitor and manage onboarded school tenants.":<>Manage <strong>{me?.school_name||school?.name||"your school"}</strong>.</>}</p></div>
    <div className="status"><span/> {isPlatform?"Platform operations":me?.school_name||school?.name||"School workspace"}</div>
   </header>}
   {error&&<div className="error banner">{error}</div>}

   {isPlatform ? <>
    <section className="stats">
     <Stat label="API database" value={monitoring?.database.status==="healthy"?1:0} detail={monitoring?.database.status==="healthy"?"Healthy":"Unavailable"}/>
     <Stat label="Total schools" value={monitoring?.schools.total??schools.length} detail="Registered tenants"/>
     <Stat label="Pending review" value={monitoring?.schools.pending??0} detail="Awaiting approval"/>
     <Stat label="Users" value={monitoring?.users??platformUsers} detail="Across all schools"/>
    </section>
    <section className="grid-2">
     <div className="panel"><div className="panel-head"><div><h2>Platform operations</h2><p>Application and tenant health from the live platform API.</p></div><span className={"pill "+(monitoring?.database.status==="healthy"?"active":"suspended")}>{monitoring?.database.status==="healthy"?"Operational":"Attention"}</span></div>
      <div className="role-list">
       <div><span>Database connectivity</span><strong>{monitoring?.database.status==="healthy"?"Healthy":"Unavailable"}</strong></div>
       <div><span>Active school tenants</span><strong>{monitoring?.schools.active??activeSchools}</strong></div>
       <div><span>Suspended tenants</span><strong>{monitoring?.schools.suspended??suspendedSchools}</strong></div>
       <div><span>Platform boundary</span><strong>Enforced</strong></div>
      </div>
     </div>
     <div className="panel"><div className="panel-head"><div><h2>Attention queue</h2><p>Tenant conditions that may require action.</p></div><Link href="/platform/schools">Open control center →</Link></div>
      <div className="table-wrap"><table><thead><tr><th>School</th><th>Signal</th><th>Status</th></tr></thead><tbody>
       {schools.filter(s=>s.status!=="active"||s.user_count===0).slice(0,6).map(s=><tr key={s.id}><td><strong>{s.name}</strong><small>{s.code}</small></td><td>{s.status==="pending"?"Approval required":s.status==="suspended"?"Suspended tenant":"No users yet"}</td><td><span className={"pill "+s.status}>{s.status}</span></td></tr>)}
      </tbody></table>{!schools.some(s=>s.status!=="active"||s.user_count===0)&&<div className="empty">No tenant issues detected.</div>}</div>
     </div>
    </section>
    <section className="panel"><div className="panel-head"><div><h2>Tenant monitoring</h2><p>Monitor every school without entering its tenant workspace.</p></div><div><span className="muted">{lastUpdated?"Last checked "+lastUpdated.toLocaleTimeString():"Checking…"}</span> <button className="ghost" onClick={()=>window.location.reload()}>Refresh</button></div></div>
     <div className="table-wrap"><table><thead><tr><th>School</th><th>Code</th><th>Users</th><th>Status</th><th>Signal</th></tr></thead><tbody>
      {schools.map(s=>{const signal=s.status==="pending"?"Awaiting approval":s.status==="suspended"?"Access blocked":s.user_count===0?"No users provisioned":"Operational";return <tr key={s.id}><td><strong>{s.name}</strong></td><td>{s.code}</td><td>{s.user_count}</td><td><span className={"pill "+s.status}>{s.status}</span></td><td>{signal}</td></tr>})}
     </tbody></table>{!schools.length&&<div className="empty">No school tenants onboarded yet.</div>}</div>
    </section>
    <section className="grid-2">
     <div className="panel"><div className="panel-head"><div><h2>Recent platform activity</h2><p>Latest audit events across the system.</p></div></div>
      <div className="table-wrap"><table><thead><tr><th>Action</th><th>Resource</th><th>Time</th></tr></thead><tbody>
       {(monitoring?.activity||[]).map((a,i)=><tr key={i}><td><strong>{a.action}</strong></td><td>{a.resource}</td><td>{new Date(a.created_at).toLocaleString()}</td></tr>)}
      </tbody></table>{!monitoring?.activity?.length&&<div className="empty">No recent audit activity.</div>}</div>
     </div>
     <div className="panel"><div className="panel-head"><div><h2>Control center</h2><p>Take action when monitoring identifies a tenant that needs attention.</p></div></div>
      <div className="role-list"><div><span>Last API check</span><strong>{monitoring?.checked_at?new Date(monitoring.checked_at).toLocaleTimeString():"—"}</strong></div><div><span>Refresh interval</span><strong>30 seconds</strong></div><div><span>Infrastructure metrics</span><strong>Render</strong></div><div><span>Tenant management</span><Link href="/platform/schools">Open →</Link></div></div>
     </div>
    </section>
   </>: <>
    <section className="stats"><Stat label="Students" value={students.length} detail="Registered profiles"/><Stat label="Teachers" value={users.filter(u=>u.role==="teacher").length} detail="Teacher accounts"/><Stat label="Parents" value={users.filter(u=>u.role==="parent").length} detail="Parent accounts"/><Stat label="Classes" value={classes.length} detail="Configured classes"/></section>
    <section className="grid-2">
      <div className="panel"><div className="panel-head"><div><h2>School readiness</h2><p>Complete these foundations before daily operations.</p></div><Link href="/dashboard/academic">Manage academic setup →</Link></div>
        <div className="role-list">
          <div><span>School profile</span><strong>{school?.status==="active"?"Ready":"Attention"}</strong></div>
          <div><span>Active academic session</span><strong>{sessions.some(s=>s.status==="active")?"Ready":"Needs setup"}</strong></div>
          <div><span>Classes configured</span><strong>{classes.length?"Ready":"Needs setup"}</strong></div>
          <div><span>Subjects configured</span><strong>{subjects.length?"Ready":"Needs setup"}</strong></div>
          <div><span>Teacher coverage</span><strong><Link href="/dashboard/teacher-assignments">Review →</Link></strong></div>
        </div>
      </div>
      <div className="panel"><div className="panel-head"><div><h2>Quick actions</h2><p>Common school-administration tasks.</p></div></div>
        <div className="module-grid">
          <Link className="module" href="/dashboard/onboarding"><div className="module-icon">+</div><div><strong>Onboard people</strong><p>Add teachers, students and parents</p></div><span>Open</span></Link>
          <Link className="module" href="/dashboard/enrollments"><div className="module-icon">E</div><div><strong>Enroll students</strong><p>Assign students to sessions and classes</p></div><span>Open</span></Link>
          <Link className="module" href="/dashboard/attendance"><div className="module-icon">A</div><div><strong>Attendance</strong><p>Start daily attendance workflows</p></div><span>Open</span></Link>
          <Link className="module" href="/dashboard/results"><div className="module-icon">R</div><div><strong>Results</strong><p>Review academic results and reports</p></div><span>Open</span></Link>
        </div>
      </div>
    </section>
    <section className="grid-2"><div className="panel"><div className="panel-head"><div><h2>Recent students</h2><p>Latest student records</p></div><Link href="/dashboard/students">View all →</Link></div><div className="table-wrap"><table><thead><tr><th>Student</th><th>Admission</th><th>Status</th></tr></thead><tbody>{students.slice(0,6).map(s=><tr key={s.id}><td><strong>{s.user?.name||"Student"}</strong><small>{s.user?.email||""}</small></td><td>{s.admission_number}</td><td><span className={"pill "+s.enrollment_status}>{s.enrollment_status}</span></td></tr>)}</tbody></table>{!students.length&&<div className="empty">No students yet.</div>}</div></div><div className="panel"><div className="panel-head"><div><h2>Access snapshot</h2><p>Current role distribution</p></div></div><div className="role-list">{["teacher","student","parent","accountant","staff"].map(role=><div key={role}><span>{role.replace("_"," ")}</span><strong>{users.filter(u=>u.role===role).length}</strong></div>)}</div></div></section>
   </>}
  </main>
 </div>
}
function Stat(p:{label:string;value:number;detail:string}){return <div className="stat"><span>{p.label}</span><strong>{p.value}</strong><small>{p.detail}</small></div>}
