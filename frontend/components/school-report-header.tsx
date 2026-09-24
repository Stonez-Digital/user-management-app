"use client";

import {useEffect,useState} from "react";

type Me={school_id?:string|null;school_name?:string|null;school_logo_url?:string|null};

export default function SchoolReportHeader({subtitle="Official academic report"}:{subtitle?:string}){
 const [me,setMe]=useState<Me|null>(null);
 useEffect(()=>{
  const token=localStorage.getItem("access_token");
  if(!token)return;
  fetch("/backend/me",{headers:{Authorization:"Bearer "+token}})
   .then(r=>r.ok?r.json():null).then(v=>v&&setMe(v)).catch(()=>{});
 },[]);
 if(!me?.school_id)return null;
 return <section className="school-report-header">
  {me.school_logo_url&&<img src={me.school_logo_url} alt={`${me.school_name||"School"} logo`} />}
  <div><h2>{me.school_name||"School"}</h2><p>{subtitle}</p></div>
 </section>;
}
