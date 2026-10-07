# Builds the slides as a PDF on the organisers' template artwork: run it in a folder holding the
# template's ppt/media images, docs/img/race.png as race.png and the Roboto woff2 files, then print
# deck.html to PDF with headless Chrome (--print-to-pdf, no header or footer).
# organisers' template artwork, by laying them out in HTML and printing with headless Chrome.
E = 914400.0
def box(x, y, w, h): return f"left:{x/E:.4f}in;top:{y/E:.4f}in;width:{w/E:.4f}in;height:{h/E:.4f}in"
def img(src, x, y, w, h): return f'<img src="{src}" style="position:absolute;{box(x,y,w,h)}">'
content_chrome = (img('image9.png', 0, 394226, 311700, 674100) + img('image6.png', 0, 4749797, 9144000, 393600)
                  + img('image3.png', 2905989, 4827865, 3332021, 278286) + img('image7.png', 311700, 4771154, 584516, 345986)
                  + img('image1.png', 7026412, 4790989, 1446045, 321343))
def content(n, title, body):
    return (f'<section class="slide">{content_chrome}<h2 style="position:absolute;{box(311700,445025,8520600,572700)}">{title}</h2>'
            f'{body}<div class="num">{n}</div></section>')
REPO = 'https://github.com/kinnuworks/sunflower-enact'
title = (f'<section class="slide dark">{img("image5.jpg", 1115, 0, 9155849, 5151422)}{img("image3.png", 6657348, 1505461, 2272593, 189804)}'
         f'{img("image8.png", 6657348, 187745, 2063598, 1295458)}{img("image2.png", 7595858, 4726170, 1407814, 312848)}'
         f'<h1 style="position:absolute;left:{464100/E+0.1:.3f}in;top:{3098125/E+0.1:.3f}in">Sunflower</h1>'
         f'<p class="sub" style="position:absolute;left:{464100/E+0.1:.3f}in;top:{3794075/E+0.12:.3f}in;width:7.6in">Moves a running app to the computer with the cleanest power, without losing a request.'
         f'<span>Challenge 3, ENACT: Kubernetes Dynamic Adaptation</span></p></section>')
repo = content(2, 'GitHub repo', f'''<div class="body" style="position:absolute;{box(311700,1152475,8520600,3416400)}">
  <p class="lead"><b>GitHub repo:</b> <a href="{REPO}">github.com/kinnuworks/sunflower-enact</a></p>
  <ul>
   <li><b>Start with the README:</b> the result, the challenge checklist with a screenshot of every SDK step, and how to run it.</li>
   <li><b>A recorded run you can play in a browser:</b> docs/demo, drawn from the measured data, unedited.</li>
   <li><b>The raw results:</b> evidence/ holds the file behind every number I quote.</li>
   <li><b>What I am giving back to ENACT:</b> docs/field-report.md, 45 findings, each with the fix I used.</li>
   <li>Licence: Apache-2.0</li>
  </ul></div>''')
summary = content(3, 'Summary', f'''<div class="body small" style="position:absolute;{box(311700,1152475,3830000,3416400)}"><ul>
   <li>ENACT’s policy operator names the best machine for an app and keeps that answer current as the grid changes.</li>
   <li><b>After deploy day the app does not follow.</b> The SDK pins it once, so when its machine’s power turns dirty, GreenCharge stays there, below its own 60% rule.</li>
   <li><b>Sunflower is a small controller that reads ENACT’s choice and moves the app to it,</b> once the change has lasted, without losing a request.</li>
   <li>I completed all five items of the challenge checklist in the ENACT SDK, dataspace included, then built Sunflower on top.</li>
  </ul></div>
  <img src="race.png" style="position:absolute;left:4.76in;top:1.35in;width:4.9in;border:1px solid #2a2017">
  <p class="caption" style="position:absolute;left:4.76in;top:4.14in;width:4.9in">One real run on the challenge cluster. Left: the standard build, 6 hours on dirty power at real speed. Right: with Sunflower, about 24 seconds. Each was sent 507 requests and lost none.</p>''')
stats = [('0', 'requests lost', 'across 30 moves in a row, 1,777 requests, on the challenge cluster'),
         ('6 h → 24 s', 'on dirty power, at real speed', 'standard build against Sunflower, over a real day of British grid data'),
         ('45', 'findings given back', 'problems I hit in ENACT, each written up with the fix I used')]
cols = ''.join(f'<div class="stat" style="position:absolute;left:{0.341+k*3.183:.3f}in;top:1.24in;width:2.95in"><b>{a}</b><i>{b}</i><span>{c}</span></div>' for k, (a, b, c) in enumerate(stats))
highlights = content(4, 'Highlights', cols + f'''<div class="body tight" style="position:absolute;{box(311700,2560000,8520600,2060000)}"><ul>
   <li><b>How I get 6 hours against 24 seconds:</b> my replay runs the day 450 times faster and shows 48 s against 12 s. The standard build’s 48 s are the whole dirty stretch, which is 6 hours at real speed. Sunflower’s 12 s are a wait and a 3.5 s move that do not stretch with the day: with its default 20 s wait, about 24 seconds.</li>
   <li><b>Checklist done in the ENACT SDK:</b> dataspace feed, policy model, Application Controller extension (14 tests), policy, packaging, deployment and monitoring, plus one step with the SDK’s assistant. The repo has a screenshot of every step.</li>
   <li><b>ENACT stays in charge:</b> Sunflower reads the policy’s chosen machine and has no ranking of its own. It waits out short dips, ignores near-ties, limits how often it moves, and undoes a move that fails.</li>
   <li><b>What I do not claim:</b> carbon saved. This cluster reports energy as a model estimate, so I report time on dirty power, which I measured.</li>
  </ul></div>''')
css = '''
@font-face{font-family:Roboto;src:url(roboto-400.woff2);font-weight:400}@font-face{font-family:Roboto;src:url(roboto-500.woff2);font-weight:500}@font-face{font-family:Roboto;src:url(roboto-700.woff2);font-weight:700}
@page{size:10in 5.625in;margin:0}*{box-sizing:border-box}html,body{margin:0;padding:0}
body{font-family:Roboto,Arial,sans-serif;color:#434343;-webkit-print-color-adjust:exact;print-color-adjust:exact}
.slide{position:relative;width:10in;height:5.625in;overflow:hidden;page-break-after:always;background:#fff}.slide.dark{background:#000}
h1{margin:0;font-size:40pt;font-weight:700;color:#fff;line-height:1}
.sub{margin:0;color:#fff;font-size:15pt;line-height:1.25}.sub span{display:block;margin-top:9pt;font-size:12pt;color:#c9d6ea}
h2{margin:0;padding:.1in;font-size:24pt;font-weight:700;color:#25346a;line-height:1.1}
.body{padding:.1in;font-size:13pt;line-height:1.3}.body.small{font-size:12.5pt}.body.tight{font-size:11.5pt;line-height:1.28}
.body ul{margin:0;padding-left:.3in}.body li{margin-bottom:7pt}.body.tight li{margin-bottom:5pt}.body b{color:#222}
.lead{margin:0 0 12pt;font-size:14pt}.lead a{color:#25346a;font-weight:700;text-decoration:none}
.caption{margin:0;font-size:9.5pt;line-height:1.3;color:#595959}
.stat b{display:block;font-size:35pt;line-height:1;color:#25346a}.stat i{display:block;font-style:normal;font-weight:700;font-size:13.5pt;color:#2a2017;margin-top:3pt}.stat span{display:block;font-size:10.5pt;line-height:1.3;color:#595959;margin-top:3pt}
.num{position:absolute;right:.16in;bottom:.1in;font-size:10pt;color:#fff}
'''
open('deck.html', 'w').write(f'<!doctype html><meta charset="utf-8"><title>Sunflower</title><style>{css}</style>{title}{repo}{summary}{highlights}')
