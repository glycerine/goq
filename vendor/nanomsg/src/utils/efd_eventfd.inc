/*
    Copyright (c) 2012-2013 Martin Sustrik  All rights reserved.

    Permission is hereby granted, free of charge, to any person obtaining a copy
    of this software and associated documentation files (the "Software"),
    to deal in the Software without restriction, including without limitation
    the rights to use, copy, modify, merge, publish, distribute, sublicense,
    and/or sell copies of the Software, and to permit persons to whom
    the Software is furnished to do so, subject to the following conditions:

    The above copyright notice and this permission notice shall be included
    in all copies or substantial portions of the Software.

    THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
    IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
    FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
    THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
    LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
    FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
    IN THE SOFTWARE.
*/

#include "err.h"
#include "int.h"
#include "closefd.h"

#include <sys/eventfd.h>
#include <unistd.h>
#include <fcntl.h>
#include <stdlib.h>
#include <sys/types.h>

tracker_t global_track_eventfd[EVTRACKSZ];
int global_track_eventfd_len = 0;
int global_track_eventfd_initialized = 0;

void init_efdtr();

// signature needed to be registered with on_exit()
void efdtr_dump(int status, void* p) {
  init_efdtr();

  if (global_track_eventfd_len == 0) {
    printf("\n efdtr_dump with %d in the table.\n", global_track_eventfd_len);
  } else {
    printf("\n ALERT!! efdtr_dump with %d in the table.\n", global_track_eventfd_len);
  }
  int i = 0;
  for (i = 0; i < global_track_eventfd_len; i++) {
    printf("efdtr_dump eventfd: %d, allocate loc: '%s'\n", global_track_eventfd[i].fd, global_track_eventfd[i].loc);
  }
}

void init_efdtr() {
  if (global_track_eventfd_initialized == 12) {
     return;
  }

  global_track_eventfd_initialized = 12;
  on_exit(efdtr_dump, 0);
  printf("\n init_efdtr() called and we are now initializing.\n");
  bzero(&global_track_eventfd[0], sizeof(int) * EVTRACKSZ);
  global_track_eventfd_len = 0;
}

void efdtr_add(int x, char* file, int line) {
  init_efdtr();
  printf("\n [pid %d] efdtr_add(%d) called. table len = %d. loc: %s:%d\n", getpid(), x, global_track_eventfd_len, file, line);

  /* elim duplicate adds */
  int loc = 0;
  int found = 0;
  for (loc = 0; loc < global_track_eventfd_len; loc++) {
    if (global_track_eventfd[loc].fd == x) {
       return; /*dup*/
    }
  }

  if (global_track_eventfd_len >= EVTRACKSZ) {
      printf("\n warning: in efdtr_add: global_track_eventfd_len(%d) >= EVTRACKSZ(%d)\n", global_track_eventfd_len, EVTRACKSZ);
      efdtr_dump(0, 0);
  }
  nn_assert(global_track_eventfd_len < EVTRACKSZ);
  global_track_eventfd[global_track_eventfd_len].fd = x;
  snprintf(global_track_eventfd[global_track_eventfd_len].loc, 59, "at: %s(%d)", file, line);
  global_track_eventfd_len++;
}

void efdtr_del(int x) {
  init_efdtr();
  if (global_track_eventfd_len >= EVTRACKSZ) {
      printf("\n [pid %d] warning in efdtr_del(): global_track_eventfd_len(%d) >= EVTRACKSZ(%d)\n", getpid(), global_track_eventfd_len, EVTRACKSZ);
  }
  printf("\n efdtr_del(%d) called. table len = %d\n", x, global_track_eventfd_len);
  nn_assert(global_track_eventfd_len < EVTRACKSZ);
  int i = 0;
  int loc = 0;
  int found = 0;

  for (loc = 0; loc < global_track_eventfd_len; loc++) {
    if (global_track_eventfd[loc].fd == x) {
      found = 1;
      break;
    }
  }
  /* nn_assert(found); */
  if (!found) { 
   printf("\n in efdtr_del, %d not found. global_track_eventfd_len = %d\n", x, global_track_eventfd_len);
   return; 
  }

  for (i = loc+1; i < global_track_eventfd_len; i++) {
    global_track_eventfd[i-1] = global_track_eventfd[i];
  }
  global_track_eventfd_len--;
}



int nn_efd_init (struct nn_efd *self)
{
    int rc;
    int flags;

    init_efdtr();

    self->efd = eventfd (0, EFD_CLOEXEC);
    if (self->efd == -1 && (errno == EMFILE || errno == ENFILE))
        return -EMFILE;
    errno_assert (self->efd != -1);
    efdtrack(self->efd);

    flags = fcntl (self->efd, F_GETFL, 0);
    if (flags == -1)
        flags = 0;
    rc = fcntl (self->efd, F_SETFL, flags | O_NONBLOCK);
    errno_assert (rc != -1);

    return 0;
}

void nn_efd_term (struct nn_efd *self)
{
    nn_closefd (self->efd);
}

nn_fd nn_efd_getfd (struct nn_efd *self)
{
    return self->efd;
}

void nn_efd_signal (struct nn_efd *self)
{
    const uint64_t one = 1;
    ssize_t nbytes;

    nbytes = write (self->efd, &one, sizeof (one));
    errno_assert (nbytes == sizeof (one));
}

void nn_efd_unsignal (struct nn_efd *self)
{
    uint64_t count;

    /*  Extract all the signals from the eventfd. */
    ssize_t sz = read (self->efd, &count, sizeof (count));
    errno_assert (sz >= 0);
    nn_assert (sz == sizeof (count));
}

