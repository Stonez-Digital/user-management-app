"use client";
import {useEffect,useState} from "react";
import Link from "next/link";
import {useRouter} from "next/navigation";

type User={id:string;name:string;email:string;role:string;active:boolean};
type School={id:string;name:string;code:string;status:string;user_count:number};
type Student={id:string;admission_number:string;gender:string;enrollment_status:string;user?:{name:string;email:string}};

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
 const[error,setError]=useState("");

 useEffect(()=>{
  api("/me").then(async m=>{
   setMe(m);
   if(m?.role==="teacher"){router.replace("/dashboard/teacher");return}
   if(m?.role==="super_admin"){
    const d=await api("/platform/schools");
    setSchools(Array.isArray(d?.schools)?d.schools:[]);
    return;
   }
   const s=await api("/admin/school");
   setSchool(s);
   const [u,st]=await Promise.all([api("/admin/users"),api("/admin/students")]);
   setUsers(Array.isArray(u)?u:u?.users||[]);
   setStudents(Array.isArray(st)?st:st?.students||[]);
  }).catch(e=>{setError(e.message);if(e.message==="Session expired")router.push("/")});
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
    {!isPlatform&&<><Link href="/dashboard/school">School Setup</Link><Link href="/dashboard/academic">Academic</Link><Link href="/dashboard/students">Students</Link><Link href="/dashboard/enrollments">Enrollment</Link><Link href="/dashboard/teacher-assignments">Teacher Assignments</Link><Link href="/dashboard/operations">Operations</Link><Link href="/dashboard/attendance">Attendance</Link><Link href="/dashboard/assessments">Assessments</Link><Link href="/dashboard/results">Results</Link><Link href="/dashboard/notifications">Notifications</Link><Link href="/dashboard/users">Users & Roles</Link><Link href="/dashboard/audit">Audit Logs</Link></>}
   </nav>
   <div className="sidebar-bottom"><div className="mini-user"><div className="avatar">{me?.name?.[0]||"A"}</div><div><strong>{me?.name||"Administrator"}</strong><small>{isPlatform?"Stonez Digital Platform":me?.role||"school_admin"}</small></div></div><button className="ghost" onClick={logout}>Sign out</button></div>
  </aside>
  <main className="content">
   <header className="topbar">
    <div><p className="eyebrow">{isPlatform?"STONEZ DIGITAL":"SCHOOL ADMINISTRATION"}</p><h1>{isPlatform?"Stonez Digital Platform":"School overview"}</h1><p className="muted">{isPlatform?"Monitor and manage onboarded school tenants.":<>Manage <strong>{school?.name||"your school"}</strong>.</>}</p></div>
    <div className="status"><span/> {isPlatform?"Platform operations":school?.name||"School workspace"}</div>
   </header>
   {error&&<div className="error banner">{error}</div>}

   {isPlatform ? <>
    <section className="stats">
     <Stat label="Total schools" value={schools.length} detail="Registered tenants"/>
     <Stat label="Pending review" value={schools.filter(s=>s.status==="pending").length} detail="Awaiting approval"/>
     <Stat label="Active" value={activeSchools} detail="Operational tenants"/>
     <Stat label="Users" value={platformUsers} detail="Across all schools"/>
    </section>
    <section className="grid-2">
     <div className="panel"><div className="panel-head"><div><h2>Platform health</h2><p>Tenant-level operational signals from the platform database.</p></div><span className="pill active">Live</span></div>
      <div className="role-list">
       <div><span>Active schools</span><strong>{activeSchools}/{schools.length}</strong></div>
       <div><span>Schools needing attention</span><strong>{schools.filter(s=>s.status!=="active"||s.user_count===0).length}</strong></div>
       <div><span>Suspended tenants</span><strong>{suspendedSchools}</strong></div>
       <div><span>Platform boundary</span><strong>Enforced</strong></div>
      </div>
     </div>
     <div className="panel"><div className="panel-head"><div><h2>Attention queue</h2><p>Items that may require platform action.</p></div><Link href="/platform/schools">Open control center →</Link></div>
      <div className="table-wrap"><table><thead><tr><th>School</th><th>Signal</th><th>Status</th></tr></thead><tbody>
       {schools.filter(s=>s.status!=="active"||s.user_count===0).slice(0,6).map(s=><tr key={s.id}><td><strong>{s.name}</strong><small>{s.code}</small></td><td>{s.status==="pending"?"Approval required":s.status==="suspended"?"Suspended tenant":"No users yet"}</td><td><span className={"pill "+s.status}>{s.status}</span></td></tr>)}
      </tbody></table>{!schools.some(s=>s.status!=="active"||s.user_count===0)&&<div className="empty">No tenant issues detected.</div>}</div>
     </div>
    </section>
    <section className="panel"><div className="panel-head"><div><h2>Tenant monitoring</h2><p>Monitor every school without entering its tenant workspace.</p></div><button className="ghost" onClick={()=>window.location.reload()}>Refresh</button></div>
     <div className="table-wrap"><table><thead><tr><th>School</th><th>Code</th><th>Users</th><th>Tenant status</th><th>Operational signal</th></tr></thead><tbody>
      {schools.map(s=>{const signal=s.status==="pending"?"Awaiting approval":s.status==="suspended"?"Access blocked":s.user_count===0?"No users provisioned":"Operational";return <tr key={s.id}><td><strong>{s.name}</strong></td><td>{s.code}</td><td>{s.user_count}</td><td><span className={"pill "+s.status}>{s.status}</span></td><td>{signal}</td></tr>})}
     </tbody></table>{!schools.length&&<div className="empty">No school tenants onboarded yet.</div>}</div>
    </section>
    <section className="grid-2">
     <div className="panel"><div className="panel-head"><div><h2>Platform boundary</h2><p>Stonez Digital operates above the school tenant layer.</p></div></div><div className="role-list"><div><span>Platform administrator</span><strong>Super Admin</strong></div><div><span>School administrators</span><strong>Tenant scoped</strong></div><div><span>School data</span><strong>Isolated</strong></div></div></div>
     <div className="panel"><div className="panel-head"><div><h2>Control center</h2><p>Take action when monitoring identifies a tenant that needs attention.</p></div></div><Link className="module" href="/platform/schools"><div className="module-icon">+</div><div><strong>Platform Schools</strong><p>Approve, activate, suspend and provision tenants</p></div><span>Open</span></Link></div>
    </section>
   </> : <>
    <section className="stats"><Stat label="Students" value={students.length} detail="Registered profiles"/><Stat label="Users" value={users.length} detail="Accounts"/><Stat label="Active" value={users.filter(u=>u.active).length} detail="Active accounts"/><Stat label="Roles" value={new Set(users.map(u=>u.role)).size} detail="Roles represented"/></section>
    <section className="grid-2"><div className="panel"><div className="panel-head"><div><h2>Recent students</h2><p>Latest student records</p></div><Link href="/dashboard/students">View all →</Link></div><div className="table-wrap"><table><thead><tr><th>Student</th><th>Admission</th><th>Status</th></tr></thead><tbody>{students.slice(0,6).map(s=><tr key={s.id}><td><strong>{s.user?.name||"Student"}</strong><small>{s.user?.email||""}</small></td><td>{s.admission_number}</td><td><span className={"pill "+s.enrollment_status}>{s.enrollment_status}</span></td></tr>)}</tbody></table>{!students.length&&<div className="empty">No students yet.</div>}</div></div><div className="panel"><div className="panel-head"><div><h2>Access snapshot</h2><p>Current role distribution</p></div></div><div className="role-list">{["super_admin","school_admin","teacher","student","parent","accountant","staff"].map(role=><div key={role}><span>{role.replace("_"," ")}</span><strong>{users.filter(u=>u.role===role).length}</strong></div>)}</div></div></section>
    <section className="panel"><div className="panel-head"><div><h2>School modules</h2><p>Foundation ready for the next academic workflows.</p></div></div><div className="module-grid"><Link className="module" href="/dashboard/academic"><div className="module-icon">+</div><div><strong>Academic structure</strong><p>Sessions, terms, classes and subjects</p></div><span>Open</span></Link>{["Enrollment","Attendance","Results & report cards"].map((x,i)=><div className="module" key={x}><div className="module-icon">+</div><div><strong>{x}</strong><p>{["Assign students to class and section","Daily attendance tracking","Grades and report cards"][i]}</p></div><span>Next</span></div>)}</div></section>
   </>}
  </main>
 </div>
}
function Stat(p:{label:string;value:number;detail:string}){return <div className="stat"><span>{p.label}</span><strong>{p.value}</strong><small>{p.detail}</small></div>}
