"use client";

import {useEffect,useState} from "react";

type Me={school_id?:string|null;school_name?:string|null;school_logo_url?:string|null};

export default function SchoolBrand(){
 const [me,setMe]=useState<Me|null>(null);
 useEffect(()=>{
  const token=localStorage.getItem("access_token");
  if(!token)return;
  fetch("/backend/me",{headers:{Authorization:"Bearer "+token}})
   .then(r=>r.ok?r.json():null)
   .then(v=>v&&setMe(v))
   .catch(()=>{});
 },[]);
 if(!me?.school_id)return null;
 const logo=me.school_logo_url||"";
 return <div className="school-brand-bar" aria-label={me.school_name||"School"}>
  {logo ? <img src={logo} alt={`${me.school_name||"School"} logo`} className="school-brand-logo"/> : <div className="school-brand-placeholder" aria-hidden="true">{(me.school_name||"S").charAt(0).toUpperCase()}</div>}
  <div className="school-brand-copy"><strong>{me.school_name||"School"}</strong><span>School Management Portal</span></div>
 </div>;
}
