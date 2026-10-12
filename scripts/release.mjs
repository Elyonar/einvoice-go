#!/usr/bin/env node
/**
 * Prepares a release of this SDK, the way standard-version does for einvoice-js.
 *
 *   make release                       suggest the version from the commits since the last tag
 *   make release BUMP=patch            or minor, major, or an exact X.Y.Z
 *   make release DRY=1                 show the plan and change nothing
 *   node scripts/release.mjs --help    the same, without make
 *
 * It writes the new version into every file that carries it, turns the CHANGELOG's "## Unreleased"
 * heading into "## X.Y.Z (date)", regenerates the guide files that record the version, and runs the
 * checks the release workflow runs. It never commits, tags or pushes: review the diff, commit it as
 * `chore(release): X.Y.Z`, open the pull request and merge it. The release workflow then publishes
 * and creates the tag and the GitHub Release.
 *
 * Below 1.0 a feature or a fix is a patch and a breaking change is a minor; from 1.0 a fix is a
 * patch, a feature a minor and a breaking change a major. Commits follow `type(scope): subject`;
 * `type!:` or a "BREAKING CHANGE" footer marks a breaking change.
 *
 * Development tooling, not shipped. Node 18 or later, standard library only.
 */
import { execSync } from 'node:child_process';
import { readFileSync, writeFileSync } from 'node:fs';

// ---- This repository (the rest of the file is identical in every port) ----
const CONFIG = {
  name: 'github.com/elyonar/einvoice-go',
  // Each pattern captures (text before)(the version)(text after) on exactly one line.
  files: [
    { path: 'version.go', pattern: /^(const Version = ")(\d+\.\d+\.\d+)(")$/m },
  ],
  // guides/guides.json and guides/operations.json record the version.
  regenerate: ['make guides operations'],
  // What the release workflow checks.
  verify: ['make lint', 'go build ./...', 'go vet ./examples/...', 'make test guides-check operations-check'],
};
// ---- Shared ----

const HELP = `Usage: node scripts/release.mjs [patch|minor|major|X.Y.Z] [--dry-run] [--no-verify]

  (no version)   suggest the next version from the commits since the last tag
  --dry-run      show the plan and change nothing
  --no-verify    skip the checks (${CONFIG.verify.join(' && ')})`;

function fail(message) {
  console.error(`release: ${message}`);
  process.exit(1);
}

function git(args) {
  return execSync(`git ${args}`, { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] }).trim();
}

function run(command) {
  console.log(`\n$ ${command}`);
  execSync(command, { stdio: 'inherit' });
}

const SEMVER = /^(\d+)\.(\d+)\.(\d+)$/;

function parse(version) {
  const m = SEMVER.exec(version);
  if (!m) fail(`"${version}" is not a version like 1.2.3`);
  return m.slice(1).map(Number);
}

function compare(a, b) {
  const [x, y] = [parse(a), parse(b)];
  for (let i = 0; i < 3; i++) if (x[i] !== y[i]) return x[i] - y[i];
  return 0;
}

function bump(version, kind) {
  const [major, minor, patch] = parse(version);
  if (kind === 'major') return `${major + 1}.0.0`;
  if (kind === 'minor') return `${major}.${minor + 1}.0`;
  return `${major}.${minor}.${patch + 1}`;
}

// The kind of bump the commits call for, as standard-version decides it.
function suggest(version, commits) {
  const preMajor = parse(version)[0] === 0;
  const breaking = commits.some((c) => /^\w+(\([^)]*\))?!:/.test(c.subject) || /BREAKING[ -]CHANGE/.test(c.body));
  const feature = commits.some((c) => /^feat(\([^)]*\))?:/.test(c.subject));
  const fix = commits.some((c) => /^(fix|perf)(\([^)]*\))?:/.test(c.subject));
  if (breaking) return preMajor ? 'minor' : 'major';
  if (feature) return preMajor ? 'patch' : 'minor';
  if (fix) return 'patch';
  return null;
}

function readVersions() {
  return CONFIG.files.map(({ path, pattern }) => {
    const text = readFileSync(path, 'utf8');
    const matches = [...text.matchAll(new RegExp(pattern.source, `${pattern.flags.replace('g', '')}g`))];
    if (matches.length !== 1) fail(`${path}: expected exactly one version, found ${matches.length}`);
    return { path, pattern, text, version: matches[0][2] };
  });
}

function today() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

function main() {
  const args = process.argv.slice(2);
  if (args.includes('--help') || args.includes('-h')) return console.log(HELP);
  const dryRun = args.includes('--dry-run');
  const verify = !args.includes('--no-verify');
  const requested = args.find((a) => !a.startsWith('--'));

  if (!dryRun && git('status --porcelain --untracked-files=no')) {
    fail('the working tree has uncommitted changes; commit or stash them first');
  }

  // The current version, which every version file must agree on.
  const files = readVersions();
  const current = files[0].version;
  const disagree = files.filter((f) => f.version !== current);
  if (disagree.length) {
    fail(`the version files disagree: ${files.map((f) => `${f.path} ${f.version}`).join(', ')}`);
  }

  // The commits since the last release tag, merges left out.
  let tag = null;
  try {
    tag = git('describe --tags --abbrev=0 --match "v[0-9]*"');
  } catch {
    // no tag yet: every commit counts
  }
  if (tag && tag !== `v${current}`) {
    console.warn(`release: warning: the last tag is ${tag} but the version files say ${current}`);
  }
  const log = git(`log ${tag ? `${tag}..HEAD` : 'HEAD'} --no-merges --format=%s%x1f%b%x1e`);
  const commits = log
    .split('\x1e')
    .map((entry) => entry.trim())
    .filter(Boolean)
    .map((entry) => {
      const [subject, body = ''] = entry.split('\x1f');
      return { subject: subject.trim(), body };
    });

  // The new version: asked for, or suggested by the commits.
  const suggested = suggest(current, commits);
  let next;
  if (!requested) {
    if (!suggested) {
      fail(`no feat, fix, perf or breaking commit since ${tag ?? 'the start'}; nothing calls for a release.\n` +
        '         Pass a version (patch, minor, major or X.Y.Z) to release anyway.');
    }
    next = bump(current, suggested);
  } else if (['patch', 'minor', 'major'].includes(requested)) {
    next = bump(current, requested);
  } else {
    parse(requested);
    next = requested;
  }
  if (compare(next, current) <= 0) fail(`${next} is not after the current version ${current}`);

  // The CHANGELOG entry: "## Unreleased", written by hand, becomes this release.
  const changelog = readFileSync('CHANGELOG.md', 'utf8');
  const heading = /^## Unreleased[^\n]*\n/m.exec(changelog);
  const rest = heading ? changelog.slice(heading.index + heading[0].length) : '';
  const nextHeading = /^## /m.exec(rest);
  const notes = (nextHeading ? rest.slice(0, nextHeading.index) : rest).trim();

  console.log(`release: ${CONFIG.name} ${current} -> ${next}` +
    (requested ? ` (asked: ${requested})` : ` (suggested: ${suggested}, from ${commits.length} commits since ${tag ?? 'the start'})`));
  console.log(`release: commits since ${tag ?? 'the start'}:`);
  for (const c of commits) console.log(`  ${c.subject}`);
  console.log(`release: version files: ${files.map((f) => f.path).join(', ')}`);

  if (!heading || !notes) {
    fail('CHANGELOG.md has no "## Unreleased" section with notes. Write what changed for users there\n' +
      '         (the commits above are a starting point), then run this again.');
  }
  if (dryRun) {
    console.log(`release: CHANGELOG.md "## Unreleased" would become "## ${next} (${today()})"`);
    console.log(`release: then: ${[...CONFIG.regenerate, ...(verify ? CONFIG.verify : [])].join(' && ')}`);
    console.log('release: dry run, nothing changed');
    return;
  }

  // Write.
  for (const f of files) {
    writeFileSync(f.path, f.text.replace(f.pattern, (_m, before, _v, after) => `${before}${next}${after}`));
  }
  writeFileSync('CHANGELOG.md',
    changelog.slice(0, heading.index) + `## ${next} (${today()})\n` + changelog.slice(heading.index + heading[0].length));

  for (const command of CONFIG.regenerate) run(command);
  if (verify) for (const command of CONFIG.verify) run(command);

  console.log(`\nrelease: ${CONFIG.name} ${next} is prepared:`);
  console.log(git('diff --stat'));
  console.log(`\nNext: review the diff, then
  git commit -am "chore(release): ${next}"
open the pull request and merge it; the release workflow publishes ${next} and creates the tag v${next}.`);
}

main();
