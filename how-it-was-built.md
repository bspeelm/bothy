---
title: How bothy was built
---

# How bothy was built

I was tired of IDEs shipping with AI bloatware and overwhelming telemetry. I mean, 1 GB for a
"lightweight" IDE is fairly laughable. But all of that would be fine if I used any of the extra
functionality that was being shipped. I used none of it. In fact I was able to narrow what I used to
three elements.

- A file browser / editor
- A console for my code assist
- A console for running CLI commands

So I did some research on what I could do in the terminal. Always being a fan of vim, I was hoping to
build around it. That ended up being not so nice. So then I went to see if there was something like a
tmux implementation out there for 2026 and I ran into Zellij and Yazi. After that, and deciding on
Ghostty for a terminal, I made my dev console alias and was off.

And it was awesome. So much faster, lighter weight and less distracting. Keeping me in the flow much
longer. So I rebuilt it on my laptop, then on my work computer. Each with a slightly different tool
stack. Which had me start wondering... can I make this truly portable?

So I started working with Claude to make an installer for my setup, and quickly started learning the
pitfalls of trying to develop a tool stack that aligned closely with the one I was using. Yes I know,
just spin up a container and blah blah blah. But that laziness had me ask a question. What if the
installer uses what's already available on any given machine, and if there is nothing useful then
makes suggestions on what to install? Then configures it.

So, the long way around, bothy was born. But it soon outgrew that small kernel of an idea and became
an effort in heavy-handed AI-assisted development. And so... after a few long days of setting
parameters and enforcement around Claude while it iterated the idea, pattern and implementation, I
ended with an admittedly over-engineered configurator.

But as is often the case the journey was more interesting than the destination. I will not belabor
the whole journey, but it is well documented in
[the decision log](https://github.com/bspeelm/bothy/blob/main/docs/decisions.md) — thirty-four ADRs,
including the ones where the answer was "don't build that" — in
[what happens when you type `bothy`](https://github.com/bspeelm/bothy/wiki/What-happens-when-you-type-bothy),
and in [the plans as they shipped](https://github.com/bspeelm/bothy/tree/main/docs/history).

So yeah, try it out. If you already have the tools, it just sets up a config and alias. If you don't,
it helps you get there. And it does it all with a fairly effortless feel and a pretty small
footprint.

If the AI-forward, or rather AI-led, approach bothers you, I get it. I am not a fan of AI slop and
this could be considered borderline, but I will say that I put a lot of care into guiding this into
what it is, and wouldn't be doing all of this if I didn't think it was at least interesting to a few
folk. I am also keenly aware that the audience for something like this is probably pretty narrow, as
anyone who cares has probably already done this. However, I like it, and if a few other people do
too, then I am happy.

---

[The documentation](https://github.com/bspeelm/bothy/wiki) ·
[the code](https://github.com/bspeelm/bothy) ·
[install it](https://github.com/bspeelm/bothy/wiki/Installing)
