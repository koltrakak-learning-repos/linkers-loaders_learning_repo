extern void a(char *);

int main(int ac, char **av) {
  static char stringa_hello[] = "Hello, world!\n";
  a(stringa_hello);
}
