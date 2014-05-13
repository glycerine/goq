
int main(int argc, char** argv) {
  // use the damn argc/argv to avoid compiler warnings.
  char* p = 0;
  if (argc > 0) {
      p = argv[argc-1];
  }
  char* a = 0;
  a[0] = *p;
  return 0;
}
