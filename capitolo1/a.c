#include <unistd.h>
#include <string.h>

unsigned int num = 4095;

void a(char *s) {
  write(1, s, strlen(s));
}
