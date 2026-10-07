# Fills the organisers' template (title, GitHub repo, Summary, Highlights) for Sunflower.
import re, shutil, subprocess, sys, zipfile, os
from xml.sax.saxutils import escape
S, R = sys.argv[1], sys.argv[2]   # a scratch folder, and the repo root
src, work, out = os.path.expanduser('~/Downloads/VelesHack_ProjectSubmissionTemplate.pptx'), f'{S}/deck', f'{R}/docs/slides/Sunflower-VelesHack.pptx'
shutil.rmtree(work, ignore_errors=True); zipfile.ZipFile(src).extractall(work)
NAVY, INK, RUST, SOFT = '25346A', '2A2017', 'B3432A', '595959'
EMU = 12700

def run(text, sz=None, b=False, color=None, i=False):
    attrs = 'lang="en"' + (f' sz="{sz}"' if sz else '') + (' b="1"' if b else '') + (' i="1"' if i else '')
    fill = f'<a:solidFill><a:srgbClr val="{color}"/></a:solidFill>' if color else ''
    face = '<a:latin typeface="Roboto"/><a:ea typeface="Roboto"/><a:cs typeface="Roboto"/><a:sym typeface="Roboto"/>'
    return f'<a:r><a:rPr {attrs}>{fill}{face}</a:rPr><a:t>{escape(text)}</a:t></a:r>'
def para(runs, bullet=False, after=600, align='l', line=110):
    b = '<a:buSzPts val="1300"/><a:buChar char="●"/>' if bullet else '<a:buNone/>'
    ind = 'indent="-285750" marL="342900"' if bullet else 'indent="0" marL="0"'
    return (f'<a:p><a:pPr {ind} lvl="0" rtl="0" algn="{align}"><a:lnSpc><a:spcPct val="{line}000"/></a:lnSpc>'
            f'<a:spcBef><a:spcPts val="0"/></a:spcBef><a:spcAft><a:spcPts val="{after}"/></a:spcAft>{b}</a:pPr>{"".join(runs)}</a:p>')
def box(i, name, x, y, w, h, paras, anchor='t'):
    return (f'<p:sp><p:nvSpPr><p:cNvPr id="{i}" name="{name}"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>'
            f'<p:spPr><a:xfrm><a:off x="{x}" y="{y}"/><a:ext cx="{w}" cy="{h}"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/><a:ln><a:noFill/></a:ln></p:spPr>'
            f'<p:txBody><a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0" anchor="{anchor}"><a:noAutofit/></a:bodyPr><a:lstStyle/>{"".join(paras)}</p:txBody></p:sp>')
def pic(i, rid, x, y, w, h, descr):
    return (f'<p:pic><p:nvPicPr><p:cNvPr id="{i}" name="Race screen" descr="{escape(descr)}"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr>'
            f'<p:blipFill><a:blip r:embed="{rid}"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>'
            f'<p:spPr><a:xfrm><a:off x="{x}" y="{y}"/><a:ext cx="{w}" cy="{h}"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:ln w="9525"><a:solidFill><a:srgbClr val="{INK}"/></a:solidFill></a:ln></p:spPr></p:pic>')

def slide(n): return open(f'{work}/ppt/slides/slide{n}.xml', encoding='utf8').read()
def save(n, x): open(f'{work}/ppt/slides/slide{n}.xml', 'w', encoding='utf8').write(x)
def set_body(x, paras, xfrm=None):
    """Replace the paragraphs of the body placeholder; optionally move and size it."""
    i = x.index('type="body"'); a = x.index('<a:lstStyle/>', i) + len('<a:lstStyle/>'); b = x.index('</p:txBody>', a)
    x = x[:a] + ''.join(paras) + x[b:]
    if xfrm:
        j = x.index('<a:xfrm>', i); k = x.index('</a:xfrm>', j)
        x = x[:j] + '<a:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/>' % xfrm + x[k:]
    return x
def add(x, shapes): return x.replace('</p:spTree>', ''.join(shapes) + '</p:spTree>')
def add_image_rel(n, target):
    p = f'{work}/ppt/slides/_rels/slide{n}.xml.rels'; r = open(p, encoding='utf8').read()
    r = r.replace('</Relationships>', f'<Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/{target}"/></Relationships>')
    open(p, 'w', encoding='utf8').write(r)

# the picture: the race screen, scaled down so the file stays small
img = f'{work}/ppt/media/race.png'
shutil.copy(f'{R}/docs/img/race.png', img); subprocess.run(['sips', '-Z', '1800', img], capture_output=True)
ratio = 1600 / 2880

# 1. title
x = slide(1)
x = x.replace('<a:t>Project_Name</a:t>', '<a:t>Sunflower</a:t>')
x = x.replace('<a:r><a:t></a:t></a:r><a:endParaRPr/></a:p></p:txBody></p:sp></p:spTree>',
              run('Moves a running app to the computer with the cleanest power, without losing a request.', sz=1500) + '</a:p>'
              + para([run('Challenge 3, ENACT: Kubernetes Dynamic Adaptation', sz=1200, color='C9D6EA')], after=0) + '</p:txBody></p:sp></p:spTree>')
x = x.replace('indent="-342900" lvl="0" marL="457200"', 'indent="0" lvl="0" marL="0"')
save(1, x)

# 2. repo
x = slide(2)
x = set_body(x, [
    para([run('GitHub repo: ', sz=1400, b=True), run('github.com/kinnuworks/sunflower-enact', sz=1400, color=NAVY, b=True)], after=900),
    para([run('Start with the README: ', sz=1300, b=True), run('the result, the challenge checklist with a screenshot of every SDK step, and how to run it.', sz=1300)], bullet=True),
    para([run('A recorded run you can play in a browser: ', sz=1300, b=True), run('docs/demo, drawn from the measured data, unedited.', sz=1300)], bullet=True),
    para([run('The raw results: ', sz=1300, b=True), run('evidence/ holds the file behind every number we quote.', sz=1300)], bullet=True),
    para([run('What we are giving back to ENACT: ', sz=1300, b=True), run('docs/field-report.md, 44 findings, each with the fix we used.', sz=1300)], bullet=True),
    para([run('Licence: Apache-2.0', sz=1300)], bullet=True, after=0),
])
save(2, x)

# 3. summary: words on the left, the screen on the right
x = slide(3)
PW = 4480000; PH = int(PW * ratio)
x = set_body(x, [
    para([run('ENACT’s policy operator names the best machine for an app and keeps that answer current as the grid changes.', sz=1300)], bullet=True),
    para([run('After deploy day the app does not follow. ', sz=1300, b=True), run('The SDK pins it once, so when its machine’s power turns dirty, GreenCharge stays there, below its own 60% rule.', sz=1300)], bullet=True),
    para([run('Sunflower is a small controller that reads ENACT’s choice and moves the app to it, ', sz=1300, b=True), run('once the change has lasted, without losing a request.', sz=1300)], bullet=True),
    para([run('We completed all five items of the challenge checklist in the ENACT SDK, dataspace included, then built Sunflower on top.', sz=1300)], bullet=True, after=0),
], xfrm=(311700, 1152475, 3830000, 3416400))
x = add(x, [pic(201, 'rId9', 4352000, 1235000, PW, PH, 'Two copies of GreenCharge over one replayed day of grid data: the standard build stays on Machine A, the copy with Sunflower moves to Machine B.'),
            box(202, 'Caption', 4352000, 1235000 + PH + 70000, PW, 420000,
                [para([run('One real run on the challenge cluster. Left: the standard build, 48 s on dirty power. Right: with Sunflower, 12 s. Each was sent 507 requests and lost none.', sz=950, color=SOFT)], after=0, line=105)])])
add_image_rel(3, 'race.png'); save(3, x)

# 4. highlights: three numbers, then what stands behind them
x = slide(4)
CW = 2700000; X0 = 311700; GAP = 210300; Y = 1180000
stats = [('0', 'requests lost', 'across 30 moves in a row, 1,777 requests, on the challenge cluster'),
         ('48 → 12 s', 'on dirty power', 'in the replay. At real speed: 6 hours for the standard build, about 24 seconds with Sunflower'),
         ('44', 'findings given back', 'problems we hit in ENACT, each written up with the fix we used')]
shapes = []
for k, (big, label, small) in enumerate(stats):
    cx = X0 + k * (CW + GAP)
    shapes.append(box(210 + k, f'Stat {k + 1}', cx, Y, CW, 1330000, [
        para([run(big, sz=3600, b=True, color=NAVY)], after=0, line=95),
        para([run(label, sz=1400, b=True, color=INK)], after=200, line=100),
        para([run(small, sz=1050, color=SOFT)], after=0, line=108)]))
x = set_body(x, [
    para([run('Where the 12 s go: ', sz=1200, b=True), run('8 s is a deliberate wait to be sure the drop is real, and 3.5 s is the move. Neither stretches with the day, so at real speed (default wait 20 s) Sunflower is on dirty power for about 24 seconds while the standard build sits through 6 hours.', sz=1200)], bullet=True, after=300),
    para([run('Checklist done in the ENACT SDK: ', sz=1200, b=True), run('dataspace feed, policy model, Application Controller extension (14 tests), policy, packaging, deployment and monitoring, plus one step with the SDK’s assistant. The repo has a screenshot of every step.', sz=1200)], bullet=True, after=300),
    para([run('ENACT stays in charge: ', sz=1200, b=True), run('Sunflower reads the policy’s chosen machine and has no ranking of its own. It waits out short dips, ignores near-ties, limits how often it moves, and undoes a move that fails.', sz=1200)], bullet=True, after=300),
    para([run('What we do not claim: ', sz=1200, b=True), run('carbon saved. This cluster reports energy as a model estimate, so we report time on dirty power, which we measured.', sz=1200)], bullet=True, after=0),
], xfrm=(311700, 2560000, 8520600, 2060000))
x = add(x, shapes); save(4, x)

if os.path.exists(out): os.remove(out)
with zipfile.ZipFile(out, 'w', zipfile.ZIP_DEFLATED) as z:
    for root, _, files in os.walk(work):
        for f in files:
            full = os.path.join(root, f); z.write(full, os.path.relpath(full, work))
print('wrote', out, os.path.getsize(out))
