# The pitch

Five minutes, strict. About 620 words spoken, which leaves room to breathe and for the screen
to do its work. Times are where you should be when each part ends.

Before you start: run `./deploy/scene.sh`, open <http://localhost:35590> full screen, and have
the slides open in a second tab. Close Eclipse, Ollama and anything else heavy.

## 0:00 to 0:40 · The problem (title slide)

> Hi, I'm ___. This is Sunflower.
>
> The challenge app, GreenCharge, sends electric-car drivers to the charger with the cleanest
> power. And ENACT's job is to run GreenCharge itself on a machine with clean power. You
> write a policy that says "at least sixty percent green", and ENACT picks the machine.
>
> We did the whole checklist through the ENACT plug-in. And then we noticed something.
> ENACT picks the machine on the day you deploy. The power grid changes every half hour.
> When that machine's power turns dirty, ENACT notices, and names a better machine. But
> nothing moves the app. It stays where it is, breaking its own rule.
>
> So we built the missing step.

## 0:40 to 3:00 · The race (switch to the race screen, press Play)

> This is live, on the challenge cluster, on this laptop. Two copies of GreenCharge. Same
> traffic to both, five requests a second. Every little stitch you see is one real request,
> and its colour is the machine that answered it. Blue is Machine A, gold is Machine B.
>
> The top chart is real data: the British power grid on the twenty-ninth of September, sped
> up so half an hour passes every four seconds. Machine A is powered by the North East of
> England, Machine B by the North West.
>
> The top copy is the standard build, exactly what the checklist gives you. The bottom copy
> has Sunflower.

*(about 0:12 on the screen's clock: ENACT's pick turns gold, Sunflower's lane shows "held")*

> There. ENACT just changed its mind, it now prefers Machine B. But Machine A is still above
> sixty percent, and B is only a little better. So Sunflower holds. It does not chase small
> differences.

*(about 0:23: the blue line crosses the dashed rule; both lanes turn red; "waiting to be sure")*

> Now Machine A has dropped below sixty percent. Both copies are breaking the rule. Sunflower
> waits a few seconds to be sure it is not a blip.

*(about 0:35: the bottom lane turns gold; the sunflower marker appears)*

> And it moves. Look at the stitches: blue, then gold, and not one gap. It started the new
> copy first, waited until it answered, and only then let the old one go. No request was lost.
>
> Now watch the two timers on the right. The standard build is still on Machine A, still
> breaking its rule, and its clock keeps running.

*(let it run to about 1:15 on the screen's clock, then talk over the rest)*

> At the end of this day of data: the standard build spent about forty-eight seconds out of
> policy. Sunflower, about thirteen. Zero requests lost on both.

## 3:00 to 3:50 · Why it is safe (stay on the race screen)

> Two things matter here. First, Sunflower never decides where the app goes. ENACT decides.
> Sunflower only carries the decision out, with the same mechanism ENACT's own plug-in uses.
> You opt an app in with one line.
>
> Second, it is careful. It ignores short dips. It ignores near-ties, because we measured
> ENACT's pick flipping back and forth between two equal machines. It limits how often it
> moves. And if a move fails, it puts the app back.
>
> We tested that: thirty moves in a row, more than two and a half thousand requests, none lost.

## 3:50 to 4:40 · What we give back (Highlights slide)

> Getting here meant reading ENACT's source code and hitting its rough edges. We wrote every
> one down, with the fix: thirty-nine findings. For example, on a fresh cluster the policy
> never picks a machine at all, for four separate reasons, and our setup script fixes all four.
> And the AI assistant loops forever on default settings, because its instructions are cut in
> half before the model sees them. We found the cause and the one-line fix.
>
> One thing is still open, and I'd rather say it than hide it: the dataspace step needs
> connection details we have not received yet.

## 4:40 to 5:00 · Close

> GreenCharge tells drivers where the clean power is. Sunflower makes GreenCharge go there
> itself. Everything you saw is in the repo, with the raw data behind every number. Thank you.

## If the live screen misbehaves

Say "let me show you the recorded run instead, it is the same data" and open the recording:
`python3 -m http.server 8000 --directory docs/demo`, then <http://localhost:8000>. It plays
the same picture from the saved measurements. Do not debug on the call.

## Words you might be asked about

| Word | Plain meaning |
|---|---|
| Cluster | a group of computers that run apps together; ours is three, simulated on one laptop |
| Node | one of those computers; we call them Machine A and Machine B |
| Kubernetes | the system that starts apps on those computers and keeps them running |
| Policy / RuntimePolicy | the list of rules you give ENACT: which region, how green |
| Policy operator | the ENACT part that reads the policy and names the best machine |
| Pin | telling Kubernetes "run this app on that exact machine" |
| Controller | a small program that watches for a change and reacts; Sunflower is one |
| Soft / Hard rule | Hard: never break it. Soft: prefer it |
| SDK / plug-in | ENACT's add-on for the Eclipse editor, with the wizards we clicked through |
