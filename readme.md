# avalyn
## _absurd web written in golang_

avalyn is an absurd web written in golang. it designed with principles from 
suckless.org software design

# config

there is no configuration file, so you will need to modify the code directly. 
i'm currently stuck on how to add a configuration feature, but dont worry 
because the program source code is relatively easy to understand

# usage

you have to compile the program and i provide the make file.

```
$ make init
$ make
```

and then run the program.

```
$ ./avalyn serve
```

if you feel confused, you can get help with:

```
$ ./avalyn help
```

# features
- easily back up all the posts.
- lock some content access with password.

# third-party libraries

avalyn would never exist without them, thank you.

- github.com/yuin/goldmark
- modernc.org/sqlite
- pkg.go.dev/golang.org/x/crypto/bcrypt
