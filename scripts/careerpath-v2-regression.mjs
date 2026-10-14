import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";

const root = path.resolve(process.cwd(), "..");
const page = await readFile(path.join(root, "frontend/app/dashboard/student/careerpath/page.tsx"), "utf8");

for (const dimension of [
  "Interests",
  "Personality and strengths",
  "Workplace preferences",
  "Subjects and skills",
  "Education and experience",
  "Next steps and goals",
]) {
  assert.ok(page.includes(dimension), `missing CareerPath dimension label: ${dimension}`);
}

assert.match(page, /Save progress/);
assert.match(page, /Review answers/);
assert.match(page, /Confirm and submit/);
assert.match(page, /Your progress has been saved/);
assert.match(page, /Your exploration results are ready/);
assert.match(page, /Your answers are private/);
assert.match(page, /not scientifically validated predictions/);
assert.match(page, /careerpath-v2|career-fit dimensions|What you will explore/i);

console.log("CareerPath v2 student-flow regression checks passed.");
