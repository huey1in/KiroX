DDK sdk for x86 and x64 platforms.
----------------------------------

You can check for existance of this optional package by verifying
the definition of the macro MINGW_HAS_DDK_H.

The DDK headers are from the ReactOS project, from their svn repo
svn://svn.reactos.org/reactos/trunk/reactos/include/ddk/

This is an optional SDK. Some of the headers are public domain and
some of them are under LGPL license. You can obtain the original
sources from the ReactOS project.

How to install this SDK
----------------------------------
Please simply copy the content of the include directory within the
include folder of our header-set. Most of the needed import libraries
are already generated within the crt build, so you shouldn't need
any further compilation.

