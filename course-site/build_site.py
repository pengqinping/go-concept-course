#!/usr/bin/env python3
"""Build the offline interactive Go course site from docs/course/*.md.

Run from anywhere: python3 course-site/build_site.py
Output: course-site/index.html
"""

import html
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
COURSE = ROOT / "docs" / "course"
FIGDIR = COURSE / "figures"
DEMODIR = ROOT / "examples" / "minimal"
OUT = ROOT / "course-site" / "index.html"

CHAPTERS = [
    ("01-package-module", "包、模块与构建边界"),
    ("02-value-semantics", "赋值到底复制了什么"),
    ("03-strings-maps", "字符串与 Map 的边界"),
    ("04-interface-boundary", "接口是谁定义的"),
    ("05-error-as-value", "失败是值，不是意外"),
    ("06-defer-panic", "defer 到底什么时候执行"),
    ("07-concurrency-model", "并发不是多开线程"),
    ("08-context-cancel", "取消是通知，不是命令"),
    ("09-shared-state", "共享状态怎么才安全"),
    ("10-orchestration", "并发编排与 goroutine 生命周期"),
    ("11-memory-escape", "内存去哪了"),
    ("12-io-boundary", "IO 是组合出来的"),
    ("13-json-boundary", "JSON 是另一套边界"),
    ("14-composition", "组合、嵌入与包边界"),
    ("15-http-boundary", "一次请求穿过哪些边界"),
    ("16-database-transaction", "事务的边界"),
    ("17-testing-feedback", "测试是设计反馈"),
    ("18-generics-limits", "泛型什么时候值得"),
    ("19-failure-in-production", "让程序在真实世界活下来"),
]
SLUGS = {slug for slug, _ in CHAPTERS}


def inline(text: str) -> str:
    """Render a controlled subset of inline Markdown."""
    s = html.escape(text, quote=False)
    s = re.sub(r"`([^`]+)`", r"<code>\1</code>", s)
    s = re.sub(r"\*\*([^*]+)\*\*", r"<strong>\1</strong>", s)

    def link(match: re.Match) -> str:
        label, target = match.group(1), match.group(2)
        chapter = re.search(r"(\d{2}-[a-z0-9-]+)\.md$", target)
        if chapter and chapter.group(1) in SLUGS:
            return f'<a href="#{chapter.group(1)}">{label}</a>'
        if target == "README.md":
            return f'<a href="#01-value-semantics">{label}</a>'
        return f'<a href="{target}">{label}</a>'

    s = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", link, s)
    return s


def slugify(text: str, used: set) -> str:
    base = re.sub(r"[^a-z0-9\u4e00-\u9fff]+", "-", text.lower()).strip("-") or "sec"
    name = base
    n = 2
    while name in used:
        name = f"{base}-{n}"
        n += 1
    used.add(name)
    return name


def render_figure(slug: str, title: str, body: list[str]) -> str:
    svg_path = FIGDIR / f"{slug}.svg"
    if not svg_path.exists():
        return f'<div class="missing">缺少图示：{html.escape(slug)}.svg</div>'
    svg = re.sub(r"<\?xml[^>]*\?>", "", svg_path.read_text(encoding="utf-8").strip())
    caption = " ".join(x.strip() for x in body if x.strip())
    return (
        '<figure class="concept-figure">'
        '<div class="figure-canvas">' + svg + "</div>"
        f"<figcaption><strong>{html.escape(title)}</strong>"
        + (f"<span>{inline(caption)}</span>" if caption else "")
        + "</figcaption></figure>"
    )


def render_demo(slug: str, title: str, body: list[str]) -> str:
    main_path = DEMODIR / slug / "main.go"
    if not main_path.exists():
        return f'<div class="missing">缺少 Demo：examples/minimal/{html.escape(slug)}/main.go</div>'
    code = main_path.read_text(encoding="utf-8").strip()
    text = " ".join(x.strip() for x in body if x.strip())
    blocks = re.split(r"(?<=[。？！])", text)
    predict, verify = "", ""
    for block in blocks:
        if block.strip().startswith("先预测"):
            predict += block.strip()
        elif block.strip().startswith("验证"):
            verify += block.strip()
    answer = verify or text
    return (
        '<div class="demo-block">'
        f'<div class="demo-head"><span class="demo-tag">动手验证</span><strong>{html.escape(title)}</strong></div>'
        + (f'<p class="predict"><span>先预测</span>{inline(predict[3:])}</p>' if predict else "")
        + f'<p class="run-hint">运行：<code>cd examples/minimal/{slug} &amp;&amp; go run .</code>（标有「可调参数」的常量就是旋钮）</p>'
        + '<div class="code-wrap"><span class="code-lang">go</span><pre><code>'
        + html.escape(code)
        + "</code></pre></div>"
        + '<details class="demo-answer"><summary>揭晓 / 核对</summary><div>'
        + inline(answer)
        + '<p class="hint">先运行 demo 核对预测，再对照正文「容易误判的地方」检查自己的模型。</p>'
        + "</div></details></div>"
    )


def render_markdown(text: str) -> tuple[str, str, dict[str, str]]:
    lines = text.splitlines()
    out: list[str] = []
    headings: dict[str, str] = {}
    used: set[str] = set()
    title = ""
    i = 0
    while i < len(lines):
        line = lines[i].rstrip()
        if not line.strip():
            i += 1
            continue
        if line.startswith("```"):
            lang = line[3:].strip()
            i += 1
            code: list[str] = []
            while i < len(lines) and not lines[i].startswith("```"):
                code.append(lines[i])
                i += 1
            i += 1
            out.append(
                '<div class="code-wrap">'
                + (f'<span class="code-lang">{html.escape(lang)}</span>' if lang else "")
                + "<pre><code>" + html.escape("\n".join(code)) + "</code></pre></div>"
            )
            continue
        if line.startswith(":::figure ") or line.startswith(":::demo "):
            kind, rest = line[3:].strip().split(None, 1)
            slug, _, label = rest.partition("|")
            i += 1
            body: list[str] = []
            while i < len(lines) and lines[i].strip() != ":::":
                body.append(lines[i])
                i += 1
            i += 1
            out.append(
                render_figure(slug.strip(), label.strip() or slug, body)
                if kind == "figure"
                else render_demo(slug.strip(), label.strip() or slug, body)
            )
            continue
        if line.startswith("# "):
            title = line[2:].strip()
            i += 1
            continue
        if line.startswith("## "):
            heading = line[3:].strip()
            hid = slugify(heading, used)
            headings[hid] = heading
            out.append(f'<h2 id="{hid}">{inline(heading)}</h2>')
            i += 1
            continue
        if line.startswith("### "):
            out.append(f"<h3>{inline(line[4:].strip())}</h3>")
            i += 1
            continue
        if line.startswith("> "):
            quote: list[str] = []
            while i < len(lines) and lines[i].startswith("> "):
                quote.append(lines[i][2:].strip())
                i += 1
            joined = " ".join(quote)
            cls = "model" if joined.startswith("一句话模型") else "note"
            out.append(f'<blockquote class="{cls}">{inline(joined)}</blockquote>')
            continue
        if line.startswith("| ") and i + 1 < len(lines) and re.match(r"^\|[\s:|-]+\|$", lines[i + 1].strip()):
            rows: list[list[str]] = []
            while i < len(lines) and lines[i].strip().startswith("|"):
                rows.append([c.strip() for c in lines[i].strip().strip("|").split("|")])
                i += 1
            head, body_rows = rows[0], rows[2:]
            out.append(
                '<div class="table-wrap"><table><thead><tr>'
                + "".join(f"<th>{inline(c)}</th>" for c in head)
                + "</tr></thead><tbody>"
                + "".join("<tr>" + "".join(f"<td>{inline(c)}</td>" for c in row) + "</tr>" for row in body_rows)
                + "</tbody></table></div>"
            )
            continue
        if re.match(r"^\d+\. ", line):
            items: list[str] = []
            while i < len(lines) and re.match(r"^\d+\. ", lines[i]):
                items.append(re.sub(r"^\d+\. ", "", lines[i]))
                i += 1
            out.append("<ol>" + "".join(f"<li>{inline(item)}</li>" for item in items) + "</ol>")
            continue
        if line.startswith("- "):
            items = []
            while i < len(lines) and lines[i].startswith("- "):
                items.append(lines[i][2:])
                i += 1
            out.append("<ul>" + "".join(f"<li>{inline(item)}</li>" for item in items) + "</ul>")
            continue
        para: list[str] = []
        while i < len(lines) and lines[i].strip() and not re.match(r"^(```|:::|#|> |\| |\d+\. |- )", lines[i]):
            para.append(lines[i].strip())
            i += 1
        if para:
            out.append(f"<p>{inline(' '.join(para))}</p>")
        else:
            i += 1
    return title, "\n".join(out), headings


CSS = """
:root{--paper:#f6f4ec;--card:#fffdf7;--ink:#20281d;--muted:#5d6857;--green:#3f6b2a;--green-soft:#e6efda;--amber:#a7631d;--line:#d8d3c5;--shadow:0 8px 28px rgb(50 58 40 / 7%)}
*{box-sizing:border-box}html{scroll-behavior:smooth}body{margin:0;background:var(--paper);color:var(--ink);font:16px/1.75 system-ui,-apple-system,"PingFang SC","Microsoft YaHei",sans-serif}
a{color:var(--green);text-underline-offset:.2em}a:hover{color:var(--amber)}
.layout{display:grid;grid-template-columns:288px minmax(0,1fr);min-height:100vh}
.sidebar{position:sticky;top:0;height:100vh;overflow:auto;padding:28px 20px;background:#eff0e6;border-right:1px solid var(--line)}
.brand{font-size:15px;font-weight:700;line-height:1.4;margin:0 0 4px}.brand small{display:block;color:var(--muted);font-size:12px;font-weight:400}
.nav{display:grid;gap:2px;margin:22px 0}.nav a{display:flex;gap:10px;padding:7px 9px;border-radius:8px;color:var(--ink);text-decoration:none;font-size:13px}
.nav a span{color:var(--muted);font-variant-numeric:tabular-nums}.nav a:hover{background:#e4e8d8}.nav a.active{background:var(--green);color:#fff}.nav a.active span{color:#dceccb}
.side-foot{font-size:12px;color:var(--muted);border-top:1px solid var(--line);padding-top:14px}
main{min-width:0;padding:40px clamp(20px,5vw,72px) 90px}.paper{max-width:880px;margin:0 auto}
.chapter{display:none}.chapter.active{display:block}
.crumb{color:var(--muted);font-size:13px;margin-bottom:6px}h1{font-size:clamp(28px,4vw,42px);line-height:1.2;margin:0 0 20px;letter-spacing:-.02em}
h2{font-size:22px;line-height:1.4;margin:42px 0 12px;padding-bottom:8px;border-bottom:1px solid var(--line)}
h3{font-size:17px;margin:26px 0 8px}p{margin:12px 0}
blockquote{margin:20px 0;padding:14px 18px;border-left:4px solid var(--green);background:var(--green-soft);border-radius:0 10px 10px 0}
blockquote.model{font-size:17px;line-height:1.7}blockquote.note{border-color:var(--amber);background:#f8ecd9}
code{font:.9em ui-monospace,SFMono-Regular,Consolas,monospace;background:#e9eddf;padding:.12em .35em;border-radius:4px}
.code-wrap{position:relative;margin:18px 0}.code-lang{position:absolute;top:6px;right:12px;color:#a9bda3;font:11px ui-monospace,monospace}
pre{margin:0;padding:18px;overflow:auto;border-radius:12px;background:#263021;color:#edf2e5;font:13px/1.7 ui-monospace,SFMono-Regular,Consolas,monospace}
pre code{background:none;padding:0;color:inherit}
ul,ol{padding-left:24px}li{margin:7px 0}
.table-wrap{overflow:auto;margin:18px 0}table{border-collapse:collapse;width:100%;font-size:14px;background:var(--card);border-radius:10px;overflow:hidden}
th,td{padding:10px 12px;text-align:left;border-bottom:1px solid var(--line);vertical-align:top}th{background:#e9eddf;font-weight:600}
.concept-figure{margin:26px 0;padding:0;background:var(--card);border:1px solid var(--line);border-radius:14px;overflow:hidden;box-shadow:var(--shadow)}
.figure-canvas{padding:12px;background:#fff}.figure-canvas svg{display:block;width:100%;height:auto}
figcaption{padding:14px 18px;border-top:1px solid var(--line);font-size:14px;color:var(--muted)}figcaption strong{display:block;color:var(--ink);font-size:15px;margin-bottom:4px}
.demo-block{margin:26px 0;border:1px solid var(--line);border-radius:14px;overflow:hidden;background:var(--card);box-shadow:var(--shadow)}
.demo-head{display:flex;gap:10px;align-items:center;padding:14px 18px;background:#eef2e5;font-size:15px}.demo-tag{font-size:11px;letter-spacing:.08em;color:#fff;background:var(--green);padding:3px 7px;border-radius:99px}
.predict{margin:0;padding:12px 18px;font-size:14px;background:#fbf4e4}.predict span{display:inline-block;margin-right:8px;padding:2px 7px;border-radius:99px;background:var(--amber);color:#fff;font-size:11px}
.run-hint{margin:0;padding:10px 18px;font-size:13px;color:var(--muted);border-bottom:1px solid var(--line)}
.demo-answer{padding:12px 18px;font-size:14px;color:var(--muted)}.demo-answer summary{cursor:pointer;color:var(--green);font-weight:600}.demo-answer[open] summary{margin-bottom:8px}.hint{margin-top:10px;font-size:13px}
.missing{padding:16px;background:#fce8e8;border-radius:10px;color:#8b2424}
.pager{display:flex;justify-content:space-between;gap:12px;margin:50px 0 0;padding-top:20px;border-top:1px solid var(--line);font-size:14px}
.pager a{padding:8px 14px;border:1px solid var(--line);border-radius:99px;background:var(--card);text-decoration:none}
@media(max-width:820px){.layout{display:block}.sidebar{position:static;height:auto;border-right:0;border-bottom:1px solid var(--line)}.nav{grid-auto-flow:column;grid-auto-columns:max-content;overflow:auto;padding-bottom:4px}.side-foot{display:none}main{padding:24px 16px 60px}}
@media print{.layout{display:block}.sidebar{display:none}main{padding:0}.chapter{display:block!important;page-break-before:always}.pager{display:none}}
"""


def main() -> None:
    chapters_html = []
    nav = []
    for idx, (slug, label) in enumerate(CHAPTERS):
        path = COURSE / f"{slug}.md"
        if not path.exists():
            raise SystemExit(f"missing chapter: {path}")
        title, body, _ = render_markdown(path.read_text(encoding="utf-8"))
        prev = f'<a href="#{CHAPTERS[idx - 1][0]}">← {CHAPTERS[idx - 1][1]}</a>' if idx else "<span></span>"
        nxt = (
            f'<a href="#{CHAPTERS[idx + 1][0]}">{CHAPTERS[idx + 1][1]} →</a>'
            if idx + 1 < len(CHAPTERS)
            else "<span></span>"
        )
        chapters_html.append(
            f'<section class="chapter" id="{slug}" data-index="{idx}">'
            f'<div class="crumb">Go 概念课 · {idx + 1:02d} / {len(CHAPTERS):02d}</div>'
            f"<h1>{html.escape(title)}</h1>{body}"
            f'<div class="pager">{prev}{nxt}</div></section>'
        )
        nav.append(f'<a href="#{slug}" data-target="{slug}"><span>{idx + 1:02d}</span>{html.escape(label)}</a>')

    doc = """<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Go 概念课 · 从会用到想明白</title>
<link rel="icon" href="data:,">
<style>""" + CSS + """</style></head><body>
<div class="layout"><aside class="sidebar">
<p class="brand">Go 概念课<small>从「会用」到「想明白」</small></p>
<nav class="nav">""" + "".join(nav) + """</nav>
<p class="side-foot">19 篇 · 每篇一张图一个真 Go Demo<br>图与源码均可离线使用<br>浏览器打印可导出全书</p>
</aside><main><div class="paper">""" + "".join(chapters_html) + """</div></main></div>
<script>
const links=[...document.querySelectorAll('.nav a')];
const chapters=[...document.querySelectorAll('.chapter')];
function show(id){
  const target=chapters.find(c=>c.id===id)||chapters[0];
  chapters.forEach(c=>c.classList.toggle('active',c===target));
  links.forEach(a=>a.classList.toggle('active',a.dataset.target===target.id));
  history.replaceState(null,'','#'+target.id);
  window.scrollTo({top:0,behavior:'instant'});
}
links.forEach(a=>a.addEventListener('click',e=>{e.preventDefault();show(a.dataset.target)}));
window.addEventListener('hashchange',()=>show(location.hash.slice(1)));
show(location.hash.slice(1));
</script></body></html>"""
    OUT.write_text(doc, encoding="utf-8")
    print(f"wrote {OUT} ({len(doc):,} chars)")


if __name__ == "__main__":
    main()
