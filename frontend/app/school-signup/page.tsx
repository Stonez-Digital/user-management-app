"use client";
import {useState} from "react";

async function readResponse(response:Response){const text=await response.text();if(!text)return {};try{return JSON.parse(text)}catch{return {error:`API returned HTTP ${response.status}`}}}

export default function SchoolSignup(){
 const [form,setForm]=useState({school_name:"",school_code:"",admin_name:"",admin_email:"",admin_password:"",confirm_password:""});
 const [error,setError]=useState(""); const [success,setSuccess]=useState(""); const [assignedDomain,setAssignedDomain]=useState(""); const [loading,setLoading]=useState(false);
 function update(key:string,value:string){setForm(v=>({...v,[key]:value}))}
 async function submit(e:React.FormEvent){e.preventDefault();setError("");setSuccess("");
  if(form.admin_password!==form.confirm_password){setError("Passwords do not match");return}
  setLoading(true);
  try{
   const r=await fetch("/backend/auth/school-signup",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({school_name:form.school_name,school_code:form.school_code,admin_name:form.admin_name,admin_email:form.admin_email,admin_password:form.admin_password})});
   const d=await readResponse(r);
   if(!r.ok)throw Error(d?.error?.message||d?.error||"Unable to submit school onboarding");
   setAssignedDomain(d?.school?.domain||"");
   setSuccess("School registration submitted. Your school domain has been reserved and will become active after Stonez Digital approves the school.");
  }catch(e){setError(e instanceof Error?e.message:"Unable to submit school onboarding")}finally{setLoading(false)}
 }
 return <main className="auth-shell"><section className="auth-card auth-card-wide">
  <div className="auth-brand"><div className="brand-mark">S</div><div><p className="eyebrow">STONEZ DIGITAL</p><span className="tenant-badge">School onboarding</span></div></div>
  <h1>Register your school</h1><p className="muted">Create your school workspace request. After registration, your school receives its own Stonez Digital web address.</p>
  <div className="tenant-note"><strong>Your school starts as pending.</strong><span>The school administrator can sign in after platform approval.</span></div>
  <form onSubmit={submit} className="form">
   <label>School name<input value={form.school_name} onChange={e=>update("school_name",e.target.value)} placeholder="Greenfield Academy" required/></label>
   <label>School code<input value={form.school_code} onChange={e=>update("school_code",e.target.value.toUpperCase())} placeholder="GREENFIELD" maxLength={50} required/></label>
   <label>Administrator name<input value={form.admin_name} onChange={e=>update("admin_name",e.target.value)} placeholder="Jane Doe" required/></label>
   <label>Administrator email<input type="email" value={form.admin_email} onChange={e=>update("admin_email",e.target.value)} placeholder="admin@school.com" required/></label>
   <label>Password<input type="password" value={form.admin_password} onChange={e=>update("admin_password",e.target.value)} minLength={8} autoComplete="new-password" required/></label>
   <label>Confirm password<input type="password" value={form.confirm_password} onChange={e=>update("confirm_password",e.target.value)} minLength={8} autoComplete="new-password" required/></label>
   {error&&<div className="error">{error}</div>}{success&&<div className="success">{success}{assignedDomain&&<p><strong>Your reserved school domain:</strong> https://{assignedDomain}</p>}<p>Your school administrator can sign in after approval and domain activation.</p></div>}
   <button disabled={loading}>{loading?"Submitting...":"Submit school onboarding"}</button>
  </form>
  <div className="auth-footer"><span>Already have an approved school?</span><span>Your school domain will be available after approval.</span></div>
 </section></main>
}