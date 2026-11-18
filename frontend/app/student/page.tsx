"use client";

import {useEffect} from "react";
import {useRouter} from "next/navigation";

export default function StudentPortalRedirect(){
  const router=useRouter();
  useEffect(()=>{router.replace("/dashboard/student")},[router]);
  return <main className="content standalone"><div className="panel"><h2>Loading student portal...</h2></div></main>;
}
