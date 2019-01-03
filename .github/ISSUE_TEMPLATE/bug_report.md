---
name: Bug report
about: For odd behaviors
title: ''
labels: bug
assignees: ''

---

**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:

**Expected behavior**
A clear and concise description of what you expected to happen.

**Screenshots**
If applicable, add screenshots to help explain your problem.

**Capture Raw Resource Data**
Run the following command on the same node that `drbdtop` was ran on and paste the output. Getting this output while the issue is occurring is *critical*
`drbdsetup events2 --timestamps --statistics --now`

**Distro**
 - RHEL/SUSE/Debian
 - Version, e.g. RHEL 7

**Additional context**
Add any other context about the problem here.
