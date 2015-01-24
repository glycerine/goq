#ifndef NN_TRACK_DOWN_EFD_LEAK_INCLUDED
#define NN_TRACK_DOWN_EFD_LEAK_INCLUDED

/* jea: track down leaking eventfd */

#define EVTRACKSZ 4000
typedef struct tracker {
  int fd;
  char loc[60];
} tracker_t;
extern tracker_t global_track_eventfd[EVTRACKSZ];

#define efdtrack(x) \
    do {\
       efdtr_add(x,  __FILE__, __LINE__); \
    } while (0)

void efdtr_del(int x);
void efdtr_add(int x, char* file, int line);
void efdtr_dump(int status, void* p);

#endif
