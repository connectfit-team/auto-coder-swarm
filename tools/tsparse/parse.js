// 파일 하나가 문법에 맞는지만 본다.
//
// tsc 를 그대로 돌리면 없는 모듈까지 오류로 낸다(의존성을 받아 두지 않았다).
// 파서만 불러 문법 오류만 본다 — dart format 과 같은 자리다.
//
// .svelte 는 스크립트와 마크업을 나눠 각각 맞는 파서로 본다. Svelte 의 파서는
// 스크립트를 JS 로 읽어서, lang="ts" 의 타입 표기를 문법 오류로 낸다.
//
// 종료 코드로 두 가지를 가른다. 부르는 쪽이 이것으로 판단한다:
//   0 맞다 · 1 문법이 틀렸다(까닭을 찍는다) · 2 검사를 못 했다(파일이 없거나
//   모듈이 없다). 2 를 통과로 세면 검사가 조용히 사라진다.
const fs = require('fs');
const path = require('path');

function cannotCheck(msg) {
  console.error(msg);
  process.exit(2);
}

function fail(msg) {
  console.log(msg);
  process.exit(1);
}

// need 는 모듈이 없는 것을 문법 오류로 오해하지 않게 갈라 낸다.
function need(name) {
  try {
    return require(name);
  } catch (e) {
    cannotCheck('모듈이 없다: ' + name + ' — scripts/install-checkers.sh 를 돌려야 한다');
  }
}

const file = process.argv[2];
if (!file) {
  cannotCheck('파일 경로가 있어야 한다');
}

let text;
try {
  text = fs.readFileSync(file, 'utf8');
} catch (e) {
  cannotCheck('파일을 읽지 못했다: ' + String((e && e.message) || e));
}
const ext = path.extname(file);

// parseTS 는 조각 하나를 TypeScript 로 읽는다. 문법 오류가 있으면 그 까닭을 준다.
function parseTS(name, body) {
  const ts = need('typescript');
  const sf = ts.createSourceFile(name, body, ts.ScriptTarget.Latest, true);
  const diags = sf.parseDiagnostics || [];
  if (diags.length === 0) return null;
  const d = diags[0];
  const pos = d.start != null
    ? ts.getLineAndCharacterOfPosition(sf, d.start)
    : { line: 0, character: 0 };
  return `${pos.line + 1}:${pos.character + 1} ${ts.flattenDiagnosticMessageText(d.messageText, ' ')}`;
}

const scriptRe = /<script\b[^>]*>([\s\S]*?)<\/script>/gi;

try {
  if (ext === '.svelte') {
    // 스크립트는 TypeScript 로 읽고, 마크업은 스크립트를 비운 뒤 Svelte 로 읽는다.
    let m;
    while ((m = scriptRe.exec(text)) !== null) {
      const msg = parseTS(file + ' <script>', m[1]);
      if (msg) fail('script ' + msg);
    }
    const markup = text.replace(scriptRe, (whole, body) =>
      whole.slice(0, whole.length - body.length - '</script>'.length) +
      '\n'.repeat((body.match(/\n/g) || []).length) + '</script>');
    need('svelte/compiler').parse(markup);
  } else {
    const msg = parseTS(file, text);
    if (msg) fail(msg);
  }
} catch (e) {
  fail(String((e && e.message) || e));
}
