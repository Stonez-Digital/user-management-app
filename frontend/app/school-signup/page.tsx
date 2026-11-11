"use client";

import {useEffect,useState} from "react";

type SignupForm = {
 school_name:string; school_code:string; school_slug:string; school_type:string; requested_plan:string;
 phone:string; address:string; city:string; state:string;
 admin_name:string; admin_email:string; admin_password:string; confirm_password:string;
};

async function readResponse(response:Response){
 const text=await response.text();
 if(!text)return {};
 try{return JSON.parse(text)}catch{return {error:`API returned HTTP ${response.status}`}}
}

const allowedLogoTypes=["image/png","image/jpeg","image/webp"];
const maxLogoBytes=2*1024*1024;

function slugFromName(name:string){
 return name.trim().toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-|-$/g,"").slice(0,63);
}

export default function SchoolSignup(){
 const [step,setStep]=useState<1|2>(1);
 const [form,setForm]=useState<SignupForm>({school_name:"",school_code:"",school_slug:"",school_type:"",requested_plan:"trial",phone:"",address:"",city:"",state:"",admin_name:"",admin_email:"",admin_password:"",confirm_password:""});
 const [logo,setLogo]=useState<File|null>(null);
 const [error,setError]=useState("");
 const [success,setSuccess]=useState("");
 const [assignedDomain,setAssignedDomain]=useState("");
 const [loading,setLoading]=useState(false);

 useEffect(()=>{
  const requested=new URLSearchParams(window.location.search).get("plan");
  if(requested&&["trial","basic","professional","premium","enterprise"].includes(requested)){update("requested_plan",requested);}
 },[]);

 function update(key:keyof SignupForm,value:string){
  setForm(current=>({...current,[key]:value}));
 }

 function updateSchoolName(value:string){
  setForm(current=>({...current,school_name:value,school_slug:current.school_slug||slugFromName(value),school_code:current.school_code||slugFromName(value).replace(/-/g,"").toUpperCase().slice(0,50)}));
 }

 function selectLogo(file:File|null){
  setError("");
  if(!file){setLogo(null);return;}
  if(!allowedLogoTypes.includes(file.type)){setError("School logo must be a PNG, JPEG, or WebP image.");return;}
  if(file.size===0||file.size>maxLogoBytes){setError("School logo must be no larger than 2 MB. This matches the current secure upload limit.");return;}
  setLogo(file);
 }

 function continueToAdmin(event:React.FormEvent<HTMLFormElement>){
  event.preventDefault();
  setError("");
  if(!form.school_name.trim()||form.school_name.trim().length<2){setError("Enter your school name to continue.");return;}
  if(!/^[A-Z0-9-]{2,50}$/.test(form.school_code.trim())){setError("Use a school code with 2–50 letters, numbers, or hyphens.");return;}
  const slug=form.school_slug.trim().toLowerCase();
  if(!/^[a-z0-9](?:[a-z0-9-]{1,61}[a-z0-9])$/.test(slug)){setError("Choose a subdomain with 3–63 lowercase letters, numbers, or hyphens. It cannot start or end with a hyphen.");return;}
  if(["app","api","www","mail","smtp","ftp","admin","autodiscover"].includes(slug)){setError("That subdomain is reserved. Please choose another.");return;}
  if(!["primary","secondary","both"].includes(form.school_type)){setError("Choose the type of school.");return;}
  setStep(2);
 }

 async function submit(event:React.FormEvent<HTMLFormElement>){
  event.preventDefault();
  setError("");setSuccess("");setAssignedDomain("");
  if(form.admin_password!==form.confirm_password){setError("Passwords do not match.");return;}
  if(form.admin_password.length<8){setError("Use a password with at least 8 characters.");return;}
  setLoading(true);
  try{
   const payload=new FormData();
   for(const key of ["school_name","school_code","school_slug","school_type","requested_plan","phone","address","city","state","admin_name","admin_email","admin_password"] as const){
    const value=form[key].trim();
    payload.append(key,value);
   }
   payload.set("school_code",form.school_code.trim().toUpperCase());
   if(logo)payload.append("logo",logo,logo.name);
   const response=await fetch("/backend/auth/school-signup",{method:"POST",body:payload});
   const data=await readResponse(response);
   if(!response.ok)throw Error(data?.error?.message||data?.error||"Unable to submit school onboarding.");
   setAssignedDomain(data?.school?.domain||"");
   setSuccess("Your school registration has been submitted. Your school workspace is pending approval by Stonez Digital. The administrator can sign in after approval and domain activation.");
  }catch(err){
   setError(err instanceof Error?err.message:"Unable to submit school onboarding.");
  }finally{setLoading(false);}
 }

 return <main className="auth-shell">
  <section className="auth-card auth-card-wide">
   <div className="auth-brand"><div className="brand-mark">S</div><div><p className="eyebrow">STONEZ DIGITAL</p><span className="tenant-badge">School onboarding</span></div></div>
   <h1>Start your school workspace</h1>
   <p className="muted">Create your school account in two short steps. No payment is collected during registration.</p>
   <div aria-label="Registration progress" style={{display:"flex",gap:12,margin:"22px 0"}}>
    <div style={{flex:1,borderBottom:step===1?"3px solid #3157d5":"3px solid #16a36a",paddingBottom:10}}><strong>1. School information</strong></div>
    <div style={{flex:1,borderBottom:step===2?"3px solid #3157d5":"3px solid #d8deea",paddingBottom:10,color:step===2?"inherit":"#778198"}}><strong>2. Admin account</strong></div>
   </div>
   <div className="tenant-note"><strong>Your school starts as pending.</strong><span>Stonez Digital reviews each school before its dedicated school domain and administrator access are activated.</span></div>

   {!success&&step===1&&<form onSubmit={continueToAdmin} className="form">
    <label>School name<input value={form.school_name} onChange={event=>updateSchoolName(event.target.value)} placeholder="Greenfield Academy" minLength={2} maxLength={160} autoComplete="organization" required/></label>
    <label>School code<input value={form.school_code} onChange={event=>update("school_code",event.target.value.toUpperCase().replace(/[^A-Z0-9-]/g,""))} placeholder="GREENFIELD" minLength={2} maxLength={50} pattern="[A-Z0-9-]{2,50}" required/><span className="muted">A unique internal identifier for your school.</span></label>
    <label>Choose your school subdomain<input value={form.school_slug} onChange={event=>update("school_slug",event.target.value.toLowerCase().replace(/[^a-z0-9-]/g,"").slice(0,63))} placeholder="greenfield" minLength={3} maxLength={63} pattern="[a-z0-9][a-z0-9-]{1,61}[a-z0-9]" required/><span className="muted">{form.school_slug||"your-school"}.stonezdigital.com.ng</span><span className="muted">Your requested address is checked for availability. It becomes active only after platform approval.</span></label>
    <label>School type<select value={form.school_type} onChange={event=>update("school_type",event.target.value)} required><option value="">Select school type</option><option value="primary">Primary only</option><option value="secondary">Secondary only</option><option value="both">Primary and secondary</option></select></label>
    <label>Choose a plan<select value={form.requested_plan} onChange={event=>update("requested_plan",event.target.value)} required><option value="trial">Free Trial · 1 term</option><option value="basic">Basic · ₦65,000 per term</option><option value="professional">Professional · ₦100,000 per term</option><option value="premium">Premium · ₦150,000 per term</option><option value="enterprise">Enterprise · Contact sales</option></select><span className="muted">This records your requested plan only. Billing and plan activation are handled after review.</span></label>
    <label>Phone number <span className="muted">Optional</span><input type="tel" value={form.phone} onChange={event=>update("phone",event.target.value)} placeholder="+234..." maxLength={80} autoComplete="tel"/></label>
    <label>Address <span className="muted">Optional</span><input value={form.address} onChange={event=>update("address",event.target.value)} placeholder="Street address" maxLength={300} autoComplete="street-address"/></label>
    <div className="form-row"><label>City <span className="muted">Optional</span><input value={form.city} onChange={event=>update("city",event.target.value)} placeholder="Makurdi" maxLength={100} autoComplete="address-level2"/></label><label>State <span className="muted">Optional</span><input value={form.state} onChange={event=>update("state",event.target.value)} placeholder="Benue" maxLength={100} autoComplete="address-level1"/></label></div>
    <label>School logo <span className="muted">Optional · PNG, JPG, or WebP · Maximum 2 MB</span><input type="file" accept="image/png,image/jpeg,image/webp" onChange={event=>selectLogo(event.target.files?.[0]||null)}/>{logo&&<span className="muted">Selected: {logo.name} · {(logo.size/1024).toFixed(0)} KB</span>}</label>
    {error&&<div className="error" role="alert">{error}</div>}
    <button type="submit">Continue to admin account</button>
   </form>}

   {!success&&step===2&&<form onSubmit={submit} className="form">
    <div className="tenant-note"><strong>{form.school_name}</strong><span>{form.school_slug}.stonezdigital.com.ng · {form.school_type} · Requested plan: {form.requested_plan}</span><span>Now create the first school administrator account.</span></div>
    <label>Administrator full name<input value={form.admin_name} onChange={event=>update("admin_name",event.target.value)} placeholder="Jane Doe" minLength={2} maxLength={100} autoComplete="name" required/></label>
    <label>Administrator email<input type="email" value={form.admin_email} onChange={event=>update("admin_email",event.target.value)} placeholder="admin@school.com" maxLength={255} autoComplete="email" required/></label>
    <label>Password<input type="password" value={form.admin_password} onChange={event=>update("admin_password",event.target.value)} minLength={8} maxLength={128} autoComplete="new-password" required/></label>
    <label>Confirm password<input type="password" value={form.confirm_password} onChange={event=>update("confirm_password",event.target.value)} minLength={8} maxLength={128} autoComplete="new-password" required/></label>
    {error&&<div className="error" role="alert">{error}</div>}
    <div style={{display:"flex",gap:12,flexWrap:"wrap"}}>
     <button type="button" className="secondary-button" disabled={loading} onClick={()=>{setError("");setStep(1);}}>Back</button>
     <button type="submit" disabled={loading}>{loading?"Submitting...":"Submit school registration"}</button>
    </div>
   </form>}

   {success&&<div className="success" role="status"><strong>Registration received</strong><p>{success}</p>{assignedDomain&&<p><strong>Requested school domain:</strong> https://{assignedDomain}</p>}<p>Keep this page for your records. You do not need to register again while approval is pending.</p></div>}
   <div className="auth-footer"><span>Already have an approved school?</span><span>Use your school’s dedicated login page after activation.</span></div>
  </section>
 </main>;
}
