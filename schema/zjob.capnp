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
}


struct Zjob {
   cmd     @0: Text;
   out     @1: List(Text);
   host    @2: Text;
   stm     @3: Int64;
   etm     @4: Int64;
   elapsec @5: Int64;
   status  @6: Text;
   subtime @7: Int64;
   pid     @8: Int64;
   dir     @9: Text;
   msg     @10: JobMsg;
   workeraddr @11: Text;
   id        @12: Int64;

   # envelope
   fromname      @13: Text;
   fromaddr      @14: Text;

   toname        @15: Text;
   toaddr        @16: Text;

   signature     @17: Text;
}

struct Z {
  # Z must contain all types, as this is our
  # runtime type identification. It is a thin shim.

  union {
    nothing      @0: Int64;
    job          @1: Zjob;
  }
}
