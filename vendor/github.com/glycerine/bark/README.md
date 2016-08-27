# bark

a golang watchdog for fast detection and restart of child processes

## summary

Watch a child process and get notified immediately upon status change (such as termination). Remedies deficiency in the Go stdlib.

## relevant inspiration:

a) detection of child process failure on OSX using the
   golang stdlib os.Process.Wait can take 30 seconds. Ouch.

b) Ian Lance Taylor suggested a Wait4 approach in this
   thread, and Igor Bukanov indicates it worked well
   for his use case; I confirm here that response is rapid.

   http://grokbase.com/t/gg/golang-nuts/157dmnz8dq/go-nuts-os-process-wait-and-signals-before-process-dies
   
   https://groups.google.com/forum/#!searchin/golang-nuts/os.Process.Wait$20and$20signals$20before$20process$20dies/golang-nuts/Xr7TP_yF5dQ/ksjZzddYgpcJ

what bark provides
======================

The bark library provides the ability to monitor a child process, to automatically restart it if it fails, and to shut it down (SIGKILL or kill -9) upon request.

Simple and fast.

### use / example

~~~

import (
  "github.com/betable/bark"
)

...
watcher := NewWatchdog(nil, "/path/to/child/executable")
watcher.Start() // launchs proc, automatically restarts it

// give it a second to get started
time.Sleep(2 * time.Millisecond)

// current Process ID is available here; if 0 then
// it is between restarts and you should poll again.
pid := <-watcher.CurrentPid

// ready to stop both child and watchdog
watcher.TermChildAndStopWatchdog <- true

// wait for shutdown of child and watchdog to complete
<-watcher.Done
fmt.Print("watchdog and child pid shutdown.\n")
...
~~~

### rarely needed

Also available, the ReqStopWatchdog channel will shutdown
only the watchdog, and leave the child process unmonitored:

~~~
watcher.ReqStopWatchdog <- true
~~~

Copyright (c) 2016 Betable.com and Rubicon Media, LLC.

License: Apache 2.0
