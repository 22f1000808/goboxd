## [2026-05-24] [Context: Learning Roadmap Help]

**Prompt:**
According to te given competition provide me with a step by step learning roadmap.

**Response summary:**
First basic Go language, Linux commands, Docker, HTTP and JSON, and some prerequistes and then to focus on how Linux Isolation actually works concept like 7 Namespaces, cgroups and seccompBPF. Then to study nsjail and the structure of config file and the README.md of nsjail repo and using nsjail from GO.

**What we used / didn't use:**
I went though all concepts one by one along with their hands on in Linux system throughly also went through some lectures as well 
https://youtu.be/sK5i-N34im8?si=UsL8Wl7AroaHRlgy
 Cgroups, namespaces, and beyond: what are containers made from? 

https://youtu.be/8fi7uSYlOdc?si=00lk1qVAAWmNG56_
 Containers From Scratch • Liz Rice • GOTO 2018 




## [2026-05-26] [Context: Finding exact sequirity holes in reference Python Implemenatation]

**Prompt:**
Please go through the code_manager.py and code_runner.py and al the 7 sequirity holes mention on the spec and identify all the holes throughly and explain each holes and why it is a whole and how can we fix that ?

**Response summary:**
All sequirity Holes along with other concurrency holes and whats the concept behind this and how exactly we can patch these holes.

**What we used / didn't use:**
Went through the holes and fixed them one by one after that found 5 more another sequirity holes try to fix them as well.




## [2026-05-26] [Context: nsjail config design - template vs multiple static files ]

**Prompt:**
Can i build multiple config files for multipe severity and plug them accordingly is this productionable ?

**Response summary:**
Yes we can do, but this adds complexity and potential injection risks.

**What we used / didn't use:**
Implemented `internal/jail/template.go` renders config file per language from config.



## [2026-05-28] [Context: Process group call Implemenatation]


**Prompt:**
If nsjail create child processes inside the sandbox and the parent nsjail process is killed, do the children survive? How do I ensure all processes in the sandbox die when I cancel execution? 

**Response summary:**
nsjail uses PID namespaces, so sandbox childrens have PIDs only meaningful inside the namespaces. So all childrens get killed from the kernel when nsjail dies via SIGKILL.
**What we used / didn't use:**
Used process group kill in `internal/jail/execute_linux.go`.





## [YYYY-MM-DD] [Context: short description of the problem]

**Prompt:**
<paste either the exact prompt or a paraphrased summary that you sent>

**Response summary:**
<what the AI said — paraphrase or direct quote>

**What we used / didn't use:**
<one sentence: what you took and what you discarded, and why>

