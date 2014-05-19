@0xcc3cde6438de7ca0;

# zjob.capnp

using Cxx = import "c++.capnp";
$Cxx.namespace("zjob");

using Go = import "go.capnp"; 
$Go.package("goq");
$Go.import("goq/schema");

enum JobMsg {
  initialsubmit      @0;
  acksubmit          @1;

  requestforwork     @2;
  delegatetoworker   @3;

  shutdownworker     @4;
  ackshutdownworker  @5;

  finishedwork       @6;
  ackfinished        @7;

  shutdownserv       @8;
  ackshutdownserv    @9;

  cancelwip          @10;
  ackcancelwip       @11;

  cancelsubmit       @12;
  ackcancelsubmit    @13;

  takesnapshot       @14;
  acktakesnapshot    @15;

  resubmitnoack      @16;
  rejectbadsig       @17;

  observejobfinish   @18;
  jobfinishednotice  @19;
  jobnotknown        @20; # might already be finished and long-gone
}

struct Zjob {
   id         @0: Int64;
   msg        @1: JobMsg;
   aboutjid   @2: Int64; # who we an inquring about.

   cmd        @3: Text;
   args       @4: List(Text);
   out        @5: List(Text);
   env        @6: List(Text);
   host       @7: Text;
   stm        @8: Int64;
   etm        @9: Int64;
   elapsec    @10: Int64;
   status     @11: Text;
   subtime    @12: Int64;
   pid        @13: Int64;
   dir        @14: Text;

   # instead of from/to, identify address by role.
   submitaddr      @15: Text;
   serveraddr      @16: Text;
   workeraddr      @17: Text; # except when when we re-try at a different worker.
   finishaddr      @18: List(Text);

   signature       @19: Text;
   islocal         @20: Bool;
   arrayid         @21: Int64;
   groupid         @22: Int64;
}

struct Z {
  # Z must contain all types, as this is our
  # runtime type identification. It is a thin shim.

  union {
    nothing      @0: Int64;
    job          @1: Zjob;
  }
}
