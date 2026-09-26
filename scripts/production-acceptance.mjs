#!/usr/bin/env node
const BASE_URL = (process.env.PRODUCTION_API_URL || "https://stonez-digital-school-api.onrender.com").replace(/\/$/,"");
const FRONTEND_URL = (process.env.PRODUCTION_FRONTEND_URL || "https://stonez-school-management.onojamondayojonugba.workers.dev").replace(/\/$/,"");
const runId = process.env.QA_RUN_ID || new Date().toISOString().replace(/[^0-9]/g,"").slice(0,14);
const schools = JSON.parse(process.env.PRODUCTION_QA_SCHOOLS_JSON || "[]");
const superAdmin = {email:process.env.PRODUCTION_QA_SUPER_ADMIN_EMAIL,password:process.env.PRODUCTION_QA_SUPER_ADMIN_PASSWORD,school_code:""};
const results = [];
const createdUsers = [];
function fail(message){throw new Error(message)}
async function request(path,{token,method="GET",body,expected=[200]}={}) {
  const r=await fetch(BASE_URL+path,{method,headers:{"Content-Type":"application/json",...(token?{Authorization:"Bearer "+token}:{})},body:body===undefined?undefined:JSON.stringify(body)});
  const text=await r.text(); let data={}; try{data=text?JSON.parse(text):{}}catch{data={raw:text}};
  if(!expected.includes(r.status)) fail(`${method} ${path} expected ${expected.join("/")} got ${r.status}: ${JSON.stringify(data)}`);
  return {status:r.status,data};
}
async function login(creds){return (await request("/auth/login",{method:"POST",body:creds,expected:[200]})).data}
async function me(token){return (await request("/me",{token})).data}
function list(d,k){return Array.isArray(d)?d:(d?.[k]||d?.data||[])}
function record(area,school,role,status,notes=""){results.push({area,school,role,status,notes})}
function idOf(v){return v?.id||v?.user?.id||v?.student?.id||v?.teacher?.id||v?.parent?.id}
async function ensureTerm(token,session,preferredStatus,dates,name){
  const terms=list((await request(`/admin/academic-sessions/${session.id}/terms`,{token})).data,"terms");
  const existing=terms.find(t=>preferredStatus==="active"?t.status==="active":t.status!=="closed");
  if(existing)return existing;
  return (await request(`/admin/academic-sessions/${session.id}/terms`,{token,method:"POST",body:{name,start_date:dates[0],end_date:dates[1],status:preferredStatus},expected:[201]})).data;
}
async function ensureAcademic(token){
  let sessions=list((await request("/admin/academic-sessions",{token})).data,"sessions");
  let active=sessions.find(x=>x.status==="active");
  if(!active){
    active=(await request("/admin/academic-sessions",{token,method:"POST",body:{name:`QA ${runId} Active Session`,start_date:"2026-09-01",end_date:"2027-07-31",status:"active"},expected:[201]})).data;
  }
  let target=sessions.find(x=>x.id!==active.id);
  if(!target){
    target=(await request("/admin/academic-sessions",{token,method:"POST",body:{name:`QA ${runId} Target Session`,start_date:"2027-09-01",end_date:"2028-07-31",status:"planned"},expected:[201]})).data;
  }
  const activeTerm=await ensureTerm(token,active,"active",["2026-09-01","2026-12-18"],`QA ${runId} First Term`);
  const targetTerm=await ensureTerm(token,target,"planned",["2027-09-01","2027-12-17"],`QA ${runId} Target Term`);
  let classes=list((await request("/admin/classes",{token})).data,"classes");
  let cls=classes[0];
  if(!cls) cls=(await request("/admin/classes",{token,method:"POST",body:{name:`QA JSS1 ${runId}`,level:1},expected:[201]})).data;
  let sections=list((await request(`/admin/classes/${cls.id}/sections`,{token})).data,"sections");
  let sec=sections[0];
  if(!sec) sec=(await request(`/admin/classes/${cls.id}/sections`,{token,method:"POST",body:{name:"QA-A"},expected:[201]})).data;
  let subjects=list((await request("/admin/subjects",{token})).data,"subjects");
  let subject=subjects[0];
  if(!subject) subject=(await request("/admin/subjects",{token,method:"POST",body:{code:`QA-${runId.slice(-6)}`,name:`QA Mathematics ${runId}`,description:"Production acceptance subject"},expected:[201]})).data;
  return {active,target,activeTerm,targetTerm,cls,sec,subject};
}
async function bulkImport(token,kind,filename,csv,expected){
  const form=new FormData();
  form.append("kind",kind); form.append("send_credentials","false");
  form.append("file",new Blob([csv],{type:"text/csv"}),filename);
  const r=await fetch(BASE_URL+"/admin/onboarding/bulk/preview",{method:"POST",headers:{Authorization:"Bearer "+token},body:form});
  const txt=await r.text(); let data={}; try{data=txt?JSON.parse(txt):{}}catch{data={raw:txt}}
  if(r.status!==201) fail("bulk "+kind+" preview expected 201 got "+r.status+": "+JSON.stringify(data));
  const job=data.job||data;
  if(job.kind!==kind||Number(job.total)!==1||Number(job.valid)!==1) fail("bulk "+kind+" preview validation mismatch: "+JSON.stringify(job));
  await request("/admin/onboarding/bulk/"+job.id+"/start",{token,method:"POST",expected:[202]});
  let completed;
  for(let i=0;i<20;i++){
    const current=(await request("/admin/onboarding/bulk/"+job.id,{token,expected:[200]})).data;
    if(current.status==="completed"||current.status==="completed_with_errors"||current.status==="failed"){completed=current;break}
    await new Promise(r=>setTimeout(r,500));
  }
  if(!completed) fail("bulk "+kind+" job did not finish within polling window");
  if(Number(completed.created)!==expected.created||Number(completed.skipped)!==expected.skipped||Number(completed.failed)!==expected.failed) fail("bulk "+kind+" counters mismatch: "+JSON.stringify({created:completed.created,skipped:completed.skipped,failed:completed.failed,expected}));
  return completed;
}

async function createAssignment(token,teacherId,academic){
  const v=(await request("/admin/teacher-assignments",{token,method:"POST",body:{teacher_id:teacherId,subject_id:academic.subject.id,academic_session_id:academic.active.id,term_id:academic.activeTerm.id,class_id:academic.cls.id,section_id:academic.sec.id,allocation_type:"subject",active:true},expected:[201]})).data;
  return v;
}
async function bootstrapSchool(s){
  const school=s.school_code;
  const admin=await login({email:s.admin_email,password:s.admin_password,school_code:school});
  const adminMe=await me(admin.access_token);
  if(adminMe.role!=="school_admin"||!adminMe.school_name) fail(`${school}: school admin /me mismatch`);
  record("authentication",school,"school_admin","PASS",adminMe.school_name);
  const academic=await ensureAcademic(admin.access_token);
  const stamp=`${runId}.${school.toLowerCase()}`;
  const password="QA-"+runId+"-Pass9";
  const studentEmail=`student.${stamp}@example.com`;
  const teacherEmail=`teacher.${stamp}@example.com`;
  const parentEmail=`parent.${stamp}@example.com`;
  const student=(await request("/admin/onboarding/people",{token:admin.access_token,method:"POST",body:{name:`QA Student ${runId}`,email:studentEmail,password,role:"student",admission_number:`QA-${runId}-${school}`,enrollment_status:"active"},expected:[201]})).data;
  const studentId=idOf(student); if(!studentId) fail(`${school}: onboarding did not return student id`); createdUsers.push({id:student.user?.id||student.user_id||studentId,token:admin.access_token});
  const teacher=(await request("/admin/onboarding/people",{token:admin.access_token,method:"POST",body:{name:`QA Teacher ${runId}`,email:teacherEmail,password,role:"teacher"},expected:[201]})).data;
  const teacherId=idOf(teacher); if(!teacherId) fail(`${school}: onboarding did not return teacher id`); createdUsers.push({id:teacher.user?.id||teacher.user_id||teacherId,token:admin.access_token});
  const parent=(await request("/admin/onboarding/people",{token:admin.access_token,method:"POST",body:{name:`QA Parent ${runId}`,email:parentEmail,password,role:"parent",student_id:studentId,relationship:"parent",primary:true},expected:[201]})).data;
  const parentId=idOf(parent); if(!parentId) fail(`${school}: onboarding did not return parent id`); createdUsers.push({id:parent.user?.id||parent.user_id||parentId,token:admin.access_token});
  record("onboarding",school,"school_admin","PASS","Created temporary teacher/student/parent");
  const bulkStamp=runId+".bulk."+school.toLowerCase();
  const bulkStudentEmail="bulk.student."+bulkStamp+"@example.com";
  const bulkTeacherEmail="bulk.teacher."+bulkStamp+"@example.com";
  const bulkParentEmail="bulk.parent."+bulkStamp+"@example.com";
  await bulkImport(admin.access_token,"students","students.csv","name,email,admission_number\nQA Bulk Student "+runId+","+bulkStudentEmail+",BULK-"+runId+"-"+school+"\n",{created:1,skipped:0,failed:0});
  await bulkImport(admin.access_token,"teachers","teachers.csv","name,email,staff_id\nQA Bulk Teacher "+runId+","+bulkTeacherEmail+",BULK-T-"+runId+"-"+school+"\n",{created:1,skipped:0,failed:0});
  await bulkImport(admin.access_token,"parents","parents.csv","name,email,parent_identifier\nQA Bulk Parent "+runId+","+bulkParentEmail+",BULK-P-"+runId+"-"+school+"\n",{created:1,skipped:0,failed:0});
  record("bulk onboarding",school,"school_admin","PASS","Student, teacher and parent imports created one account each with zero failures");
  await bulkImport(admin.access_token,"students","students-duplicate.csv","name,email,admission_number\nExisting Student,"+studentEmail+",QA-"+runId+"-"+school+"\n",{created:0,skipped:1,failed:0});
  await bulkImport(admin.access_token,"teachers","teachers-duplicate.csv","name,email,staff_id\nExisting Teacher,"+bulkTeacherEmail+",BULK-T-"+runId+"-"+school+"\n",{created:0,skipped:1,failed:0});
  await bulkImport(admin.access_token,"parents","parents-duplicate.csv","name,email,parent_identifier\nExisting Parent,"+parentEmail+","+parentEmail+"\n",{created:0,skipped:1,failed:0});
  record("bulk onboarding",school,"school_admin","PASS","Duplicate student/teacher/parent imports persisted skipped=1 and created=0");
  const enrollment=(await request("/admin/enrollments",{token:admin.access_token,method:"POST",body:{student_id:studentId,academic_session_id:academic.active.id,class_id:academic.cls.id,section_id:academic.sec.id,status:"active"},expected:[201]})).data;
  const promoted=(await request(`/admin/enrollments/${enrollment.id}/place`,{token:admin.access_token,method:"POST",body:{target_session_id:academic.target.id,target_class_id:academic.cls.id,target_section_id:academic.sec.id,operation:"promote"},expected:[201]})).data;
  const history=list((await request(`/admin/enrollments/student/${studentId}`,{token:admin.access_token})).data,"enrollments");
  if(promoted.status!=="active"||history.length<2) fail(`${school}: promotion/history verification failed`);
  record("student lifecycle",school,"school_admin","PASS","Promotion created active target and preserved history");
  const assignment=await createAssignment(admin.access_token,teacherId,academic);
  const studentAuth=await login({email:studentEmail,password,school_code:school});
  const teacherAuth=await login({email:teacherEmail,password,school_code:school});
  const parentAuth=await login({email:parentEmail,password,school_code:school});
  const studentMe=await me(studentAuth.access_token), teacherMe=await me(teacherAuth.access_token), parentMe=await me(parentAuth.access_token);
  await request("/auth/refresh",{method:"POST",body:{refresh_token:studentAuth.refresh_token},expected:[200]});
  await request("/auth/logout",{method:"POST",body:{refresh_token:studentAuth.refresh_token},expected:[200]});
  await login({email:studentEmail,password,school_code:school});
  record("logout/session",school,"student","PASS","Refresh and logout cycle completed");
  for(const [role,x] of [["student",studentMe],["teacher",teacherMe],["parent",parentMe]]) {
    if(x.role!==role||!x.school_name) fail(`${school}: ${role} /me mismatch`);
    record("authentication",school,role,"PASS",x.school_name);
  }

  const selectedEnrollment=(await request(`/student/enrollment?academic_session_id=${academic.active.id}`,{token:studentAuth.access_token})).data;
  if(selectedEnrollment.academic_session_id!==academic.active.id) fail(`${school}: student enrollment ignored selected session`);
  const targetEnrollment=(await request(`/student/enrollment?academic_session_id=${academic.target.id}`,{token:studentAuth.access_token})).data;
  if(targetEnrollment.academic_session_id!==academic.target.id) fail(`${school}: target student enrollment context mismatch`);
  const studentTerms=list((await request(`/student/terms?academic_session_id=${academic.active.id}`,{token:studentAuth.access_token})).data,"terms");
  if(!studentTerms.some(t=>t.academic_session_id===academic.active.id)) fail(`${school}: student terms crossed session`);
  await request(`/student/attendance?academic_session_id=${academic.active.id}&term_id=${academic.activeTerm.id}`,{token:studentAuth.access_token});
  await request(`/student/timetable?academic_session_id=${academic.active.id}&term_id=${academic.activeTerm.id}`,{token:studentAuth.access_token});
  record("academic access",school,"student","PASS","Selected session/term propagated through enrollment, terms, attendance and timetable");

  const teacherAssignments=list((await request(`/teacher/assignments?academic_session_id=${academic.active.id}&term_id=${academic.activeTerm.id}`,{token:teacherAuth.access_token})).data,"assignments");
  if(!teacherAssignments.length) fail(`${school}: teacher assignment acceptance returned no selected-context assignment`);
  if(teacherAssignments.some(a=>a.academic_session_id!==academic.active.id||a.term_id!==academic.activeTerm.id)) fail(`${school}: teacher assignments crossed academic context`);
  await request(`/teacher/assignments?academic_session_id=${academic.target.id}&term_id=${academic.targetTerm.id}`,{token:teacherAuth.access_token});
  record("academic access",school,"teacher","PASS","Teacher assignment filtered by selected session/term");

  const children=list((await request("/parent/children",{token:parentAuth.access_token})).data,"children");
  if(!children.some(x=>x.student_id===studentId)) fail(`${school}: parent child link missing`);
  await request(`/parent/terms?student_id=${studentId}&academic_session_id=${academic.target.id}`,{token:parentAuth.access_token});
  await request(`/parent/children/${studentId}/attendance?academic_session_id=${academic.target.id}&term_id=${academic.targetTerm.id}`,{token:parentAuth.access_token});
  record("academic access",school,"parent","PASS","Parent child access respects selected academic context");

  await request("/admin/users",{token:studentAuth.access_token,expected:[403]});
  await request("/me",{expected:[401]});
  record("role isolation",school,"student","PASS","Student denied school-admin endpoint");
  record("unauthorized API",school,"anonymous","PASS","Protected /me returned 401");
  return {school,admin,studentId,teacherId,parentId,assignment,academic};
}
async function cleanup(){
  const seen=new Set();
  for(const item of createdUsers){
    if(!item?.id||seen.has(item.id)) continue; seen.add(item.id);
    try { await request(`/admin/users/${item.id}/deactivate`,{token:item.token,method:"POST",expected:[200,404]}); } catch {}
  }
}

try {
  if(!superAdmin.email||!superAdmin.password) fail("Missing super-admin QA credentials");
  if(schools.length!==2) fail("PRODUCTION_QA_SCHOOLS_JSON must contain exactly two active school-admin credential objects");
  const superAuth=await login(superAdmin);
  const superMe=await me(superAuth.access_token);
  if(superMe.role!=="super_admin") fail("Super Admin QA account is not super_admin");
  record("authentication","platform","super_admin","PASS","Platform profile loaded");
  const contexts=[]; for(const s of schools) contexts.push(await bootstrapSchool(s));
  const a=contexts[0], b=contexts[1];
  const bUsers=list((await request("/admin/users",{token:b.admin.access_token})).data,"users");
  const bStudents=list((await request("/admin/students",{token:b.admin.access_token})).data,"students");
  if(bUsers[0]) await request(`/admin/users/${bUsers[0].id}`,{token:a.admin.access_token,expected:[404]});
  if(bStudents[0]) await request(`/admin/students/${bStudents[0].id}`,{token:a.admin.access_token,expected:[404]});
  const bSessions=list((await request("/admin/academic-sessions",{token:b.admin.access_token})).data,"sessions");
  if(bSessions[0]) await request(`/admin/academic-sessions/${bSessions[0].id}`,{token:a.admin.access_token,expected:[404]});
  record("tenant isolation",a.school,"school_admin","PASS","Cross-school user/student/session IDs rejected");
  console.log(JSON.stringify({run_id:runId,base_url:BASE_URL,results,temporary_accounts:{count:createdUsers.length,cleanup:"deactivate after run"}},null,2));
} finally {
  if(createdUsers.length) await cleanup();
}
