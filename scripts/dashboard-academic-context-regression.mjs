import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";

const root = path.resolve(process.cwd(), "..");
const read = (file) => readFile(path.join(root, "frontend", file), "utf8");

const teacher = await read("app/dashboard/teacher/page.tsx");
assert.match(
  teacher,
  /useEffect\(\(\)=>\{if\(!selectedSessionId\|\|!selectedTermId\)return;/,
  "Teacher dashboard must not request academic data before session and term are selected"
);

const student = await read("app/dashboard/student/page.tsx");
assert.match(
  student,
  /useEffect\(\(\)=>\{if\(!sessionId\|\|!termId\)return;/,
  "Student dashboard must gate academic requests on session and term"
);

const parentDashboard = await read("app/dashboard/parent/page.tsx");
assert.match(
  parentDashboard,
  /if\(!selected\|\|!sessionId\|\|!termId\)return;/,
  "Parent dashboard must gate child academic requests on child, session and term"
);
assert.match(
  parentDashboard,
  /\/parent\/terms\?student_id=\"\+id\+\"&academic_session_id=\"\+sessionId/,
  "Parent terms request must include student and academic session"
);

const parentLegacy = await read("app/parent/page.tsx");
assert.doesNotMatch(parentLegacy, /\/parent\/terms(?!\?)/, "Legacy parent route must not call /parent/terms without context");
assert.match(parentLegacy, /router\.replace\(\"\/dashboard\/parent\")/, "Legacy parent route must redirect to the context-aware dashboard");

const studentLegacy = await read("app/student/page.tsx");
assert.doesNotMatch(studentLegacy, /\/student\/(?:enrollment|attendance|timetable|invoices|payments|terms)(?!\?)/, "Legacy student route must not call context-dependent endpoints without parameters");
assert.match(studentLegacy, /router\.replace\(\"\/dashboard\/student\")/, "Legacy student route must redirect to the context-aware dashboard");

const auth = await read("lib/auth-session.ts");
assert.match(auth, /case "parent":\s*return "\/dashboard\/parent";/, "Parent role must land on the context-aware dashboard");
assert.match(auth, /case "student":\s*return "\/dashboard\/student";/, "Student role must land on the context-aware dashboard");

console.log("Dashboard academic-context regression checks passed.");
