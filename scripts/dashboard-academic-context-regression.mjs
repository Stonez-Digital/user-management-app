import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";

const root = path.resolve(process.cwd(), "..");
const read = (file) => readFile(path.join(root, "frontend", file), "utf8");

const dashboardLayout = await read("app/dashboard/layout.tsx");
assert.doesNotMatch(dashboardLayout, /AcademicContextProvider|AcademicSelector/);

for (const file of [
  "app/dashboard/teacher/layout.tsx",
  "app/dashboard/student/layout.tsx",
  "app/dashboard/parent/layout.tsx",
]) {
  const content = await read(file);
  assert.doesNotMatch(content, /AcademicContextProvider|AcademicSelector/);
}

for (const file of [
  "app/dashboard/attendance/page.tsx",
  "app/dashboard/assessments/page.tsx",
  "app/dashboard/results/page.tsx",
  "app/dashboard/finance/page.tsx",
  "app/dashboard/timetable/page.tsx",
]) {
  const content = await read(file);
  assert.match(content, /AcademicContextProvider/);
  assert.match(content, /AcademicSelector/);
}

for (const file of [
  "app/dashboard/teacher/page.tsx",
  "app/dashboard/student/page.tsx",
  "app/dashboard/parent/page.tsx",
]) {
  const content = await read(file);
  assert.doesNotMatch(content, /useAcademicContext|AcademicContextProvider|AcademicSelector/);
}
const platformDashboard = await read("app/dashboard/page.tsx");
assert.ok(platformDashboard.includes('role === "super_admin"'));
assert.ok(platformDashboard.includes("AcademicContextProvider"));
assert.ok(platformDashboard.includes("AcademicSelector"));
assert.ok(platformDashboard.includes("/admin/dashboard"));
assert.match(platformDashboard, /loading: contextLoading/);
assert.match(platformDashboard, /if \(!contextLoading\) load\(\)/);
assert.match(platformDashboard, /first-run-setup/);
assert.match(platformDashboard, /Create academic session/);
assert.match(platformDashboard, /Add academic term/);

const academicSelector = await read("components/academic-selector.tsx");
assert.match(academicSelector, /sessions\.length === 0/);
assert.match(academicSelector, /terms\.length/);
assert.match(academicSelector, /href="\/dashboard\/academic"/);
assert.match(academicSelector, /Create academic session/);

console.log("Dashboard academic-context and first-run school setup checks passed.");


const workspaceStyles = await read("app/premium-workspace.css");
assert.match(workspaceStyles, /\.app-shell > \.sidebar\s*\{\s*display:\s*none;/);
assert.match(workspaceStyles, /\.app-shell > \.content\s*\{[^}]*margin-left:\s*0/);

console.log("Legacy module sidebar overlap regression checks passed.");

const academicPage = await read("app/dashboard/academic/page.tsx");
assert.match(academicPage, /const submitLock = useRef\(false\)/);
for (const handler of ["createSession", "createClass", "createSubject"]) {
  const start = academicPage.indexOf(`async function ${handler}`);
  const end = academicPage.indexOf("\n  async function ", start + 1);
  const body = academicPage.slice(start, end < 0 ? undefined : end);
  assert.match(body, /if \(submitLock\.current\) return/);
  assert.match(body, /submitLock\.current = true/);
  assert.match(body, /submitLock\.current = false/);
}
assert.match(academicPage, /Academic session created successfully/);
assert.match(academicPage, /Class created successfully/);
assert.match(academicPage, /Subject created successfully/);

const databaseMigrations = await readFile(path.join(root, "internal", "database", "postgres.go"), "utf8");
assert.match(databaseMigrations, /Version:43,Name:"scope_academic_session_name_uniqueness_by_school"/);
assert.match(databaseMigrations, /DROP CONSTRAINT IF EXISTS academic_sessions_name_key/);
assert.match(databaseMigrations, /uq_school_session_name/);

console.log("Academic create idempotency and tenant-scoped session uniqueness checks passed.");

