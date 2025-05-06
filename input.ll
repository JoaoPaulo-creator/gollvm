@factorial = global i32 (i32)* @factorial
@fibonacci = global i32 (i32)* @fibonacci
@sum = global i32 (i32)* @sum
@findFirstMultipleOf7 = global i32 (i32)* @findFirstMultipleOf7
@.str.0 = global [23 x i8] c"Factorial calculation:\00"
@.str.1 = global [4 x i8] c"%s\0A\00"
@.str.2 = global [4 x i8] c"%d\0A\00"
@.str.3 = global [23 x i8] c"Fibonacci calculation:\00"
@.str.4 = global [36 x i8] c"Sum calculation (using while loop):\00"
@.str.5 = global [27 x i8] c"Complex arithmetic result:\00"
@.str.6 = global [14 x i8] c"Hello, world!\00"
@.str.7 = global [20 x i8] c"Comparison results:\00"
@.str.8 = global [30 x i8] c"Nested if-else demonstration:\00"
@.str.9 = global [25 x i8] c"Both conditions are true\00"
@.str.10 = global [4 x i8] c"%s\0A\00"
@.str.11 = global [29 x i8] c"Only first condition is true\00"
@.str.12 = global [4 x i8] c"%s\0A\00"
@.str.13 = global [25 x i8] c"First condition is false\00"
@.str.14 = global [4 x i8] c"%s\0A\00"
@.str.15 = global [21 x i8] c"First multiple of 7:\00"

declare i8* @malloc(i64)

declare void @free(i8*)

declare i64 @strlen(i8*)

declare i8* @strcpy(i8*, i8*)

declare i32 @abs(i32)

declare double @pow(double, double)

declare void @exit(i32)

define i32 @main() {
entry:
	%result = alloca i32
	%z = alloca i32
	%y = alloca i32
	%x = alloca i32
	store i32 5, i32* %x
	store i32 10, i32* %y
	%0 = load i32, i32* %x
	%1 = load i32, i32* %y
	%2 = add i32 %0, %1
	store i32 %2, i32* %z
	%3 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.1, i64 0, i64 0) to i8*
	%4 = call i32 (i8*) (...) @printf(i8* %3, i8* getelementptr ([23 x i8], [23 x i8]* @.str.0, i64 0, i64 0))
	%5 = call i32 @factorial(i32 5)
	%6 = call i32 @factorial(i32 5)
	%7 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.2, i64 0, i64 0) to i8*
	%8 = call i32 (i8*) (...) @printf(i8* %7, i32 0)
	%9 = bitcast [4 x i8]* @.str.1 to i8*
	%10 = call i32 (i8*) (...) @printf(i8* %9, i8* getelementptr ([23 x i8], [23 x i8]* @.str.3, i64 0, i64 0))
	%11 = call i32 @fibonacci(i32 10)
	%12 = call i32 @fibonacci(i32 10)
	%13 = bitcast [4 x i8]* @.str.2 to i8*
	%14 = call i32 (i8*) (...) @printf(i8* %13, i32 0)
	%15 = bitcast [4 x i8]* @.str.1 to i8*
	%16 = call i32 (i8*) (...) @printf(i8* %15, i8* getelementptr ([36 x i8], [36 x i8]* @.str.4, i64 0, i64 0))
	%17 = call i32 @sum(i32 100)
	%18 = call i32 @sum(i32 100)
	%19 = bitcast [4 x i8]* @.str.2 to i8*
	%20 = call i32 (i8*) (...) @printf(i8* %19, i32 0)
	%21 = load i32, i32* %x
	%22 = load i32, i32* %y
	%23 = add i32 %21, %22
	%24 = load i32, i32* %z
	%25 = sub i32 %24, 5
	%26 = mul i32 %23, %25
	store i32 %26, i32* %result
	%27 = bitcast [4 x i8]* @.str.1 to i8*
	%28 = call i32 (i8*) (...) @printf(i8* %27, i8* getelementptr ([27 x i8], [27 x i8]* @.str.5, i64 0, i64 0))
	%29 = load i32, i32* %result
	%30 = bitcast [4 x i8]* @.str.2 to i8*
	%31 = call i32 (i8*) (...) @printf(i8* %30, i32 %29)
	%32 = bitcast [4 x i8]* @.str.1 to i8*
	%33 = call i32 (i8*) (...) @printf(i8* %32, i8* getelementptr ([14 x i8], [14 x i8]* @.str.6, i64 0, i64 0))
	%34 = bitcast [4 x i8]* @.str.1 to i8*
	%35 = call i32 (i8*) (...) @printf(i8* %34, i8* getelementptr ([20 x i8], [20 x i8]* @.str.7, i64 0, i64 0))
	%36 = load i32, i32* %x
	%37 = load i32, i32* %y
	%38 = icmp slt i32 %36, %37
	%39 = bitcast [4 x i8]* @.str.2 to i8*
	%40 = call i32 (i8*) (...) @printf(i8* %39, i1 %38)
	%41 = load i32, i32* %x
	%42 = load i32, i32* %y
	%43 = icmp sgt i32 %41, %42
	%44 = bitcast [4 x i8]* @.str.2 to i8*
	%45 = call i32 (i8*) (...) @printf(i8* %44, i1 %43)
	%46 = load i32, i32* %x
	%47 = icmp eq i32 %46, 5
	%48 = bitcast [4 x i8]* @.str.2 to i8*
	%49 = call i32 (i8*) (...) @printf(i8* %48, i1 %47)
	%50 = load i32, i32* %y
	%51 = icmp ne i32 %50, 10
	%52 = bitcast [4 x i8]* @.str.2 to i8*
	%53 = call i32 (i8*) (...) @printf(i8* %52, i1 %51)
	%54 = bitcast [4 x i8]* @.str.1 to i8*
	%55 = call i32 (i8*) (...) @printf(i8* %54, i8* getelementptr ([30 x i8], [30 x i8]* @.str.8, i64 0, i64 0))
	%56 = load i32, i32* %x
	%57 = icmp slt i32 %56, 10
	br i1 %57, label %if.then.14, label %if.else.16

if.then.14:
	br label %if.merge.15

if.merge.15:
	ret i32 0

if.else.16:
	%58 = load i32, i32* %y
	%59 = icmp sgt i32 %58, 5
	br i1 %59, label %if.then.17, label %if.else.19

if.then.17:
	br label %if.merge.18

if.merge.18:
	ret i32 0

if.else.19:
	%60 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.10, i64 0, i64 0) to i8*
	%61 = call i32 (i8*) (...) @printf(i8* %60, i8* getelementptr ([25 x i8], [25 x i8]* @.str.9, i64 0, i64 0))
	%62 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.12, i64 0, i64 0) to i8*
	%63 = call i32 (i8*) (...) @printf(i8* %62, i8* getelementptr ([29 x i8], [29 x i8]* @.str.11, i64 0, i64 0))
	%64 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.14, i64 0, i64 0) to i8*
	%65 = call i32 (i8*) (...) @printf(i8* %64, i8* getelementptr ([25 x i8], [25 x i8]* @.str.13, i64 0, i64 0))
	%66 = bitcast [4 x i8]* @.str.1 to i8*
	%67 = call i32 (i8*) (...) @printf(i8* %66, i8* getelementptr ([21 x i8], [21 x i8]* @.str.15, i64 0, i64 0))
	%68 = call i32 @findFirstMultipleOf7(i32 20)
	%69 = call i32 @findFirstMultipleOf7(i32 20)
	%70 = bitcast [4 x i8]* @.str.2 to i8*
	%71 = call i32 (i8*) (...) @printf(i8* %70, i32 0)
	br label %if.merge.18
}

define i32 @factorial() {
entry:
	%n = alloca i32
	store i32 0, i32* %n
	%0 = load i32, i32* %n
	%1 = icmp sle i32 %0, 1
	br i1 %1, label %if.then.0, label %if.merge.1

if.then.0:
	br label %if.merge.1

if.merge.1:
	%2 = load i32, i32* %n
	%3 = load i32, i32* %n
	%4 = sub i32 %3, 1
	%5 = call i32 @factorial(i32 %4)
	%6 = mul i32 %2, %5
	ret i32 %6
}

define i32 @fibonacci() {
entry:
	%n = alloca i32
	store i32 0, i32* %n
	%0 = load i32, i32* %n
	%1 = icmp sle i32 %0, 0
	br i1 %1, label %if.then.2, label %if.merge.3

if.then.2:
	br label %if.merge.3

if.merge.3:
	%2 = load i32, i32* %n
	%3 = icmp eq i32 %2, 1
	br i1 %3, label %if.then.4, label %if.merge.5

if.then.4:
	br label %if.merge.5

if.merge.5:
	%4 = load i32, i32* %n
	%5 = sub i32 %4, 1
	%6 = call i32 @fibonacci(i32 %5)
	%7 = load i32, i32* %n
	%8 = sub i32 %7, 2
	%9 = call i32 @fibonacci(i32 %8)
	%10 = add i32 (i32) %6, %9
	ret i32 0
}

define i32 @sum() {
entry:
	%i = alloca i32
	%total = alloca i32
	%n = alloca i32
	store i32 0, i32* %n
	store i32 0, i32* %total
	store i32 1, i32* %i
	br label %while.cond.6

while.cond.6:
	br i1 %2, label %while.body.7, label %while.end.8

while.body.7:
	br label %while.cond.6

while.end.8:
	%0 = load i32, i32* %i
	%1 = load i32, i32* %n
	%2 = icmp sle i32 %0, %1
	%3 = load i32, i32* %total
	%4 = load i32, i32* %total
	%5 = load i32, i32* %i
	%6 = add i32 %4, %5
	%7 = load i32, i32* %i
	%8 = load i32, i32* %i
	%9 = add i32 %8, 1
	%10 = load i32, i32* %total
	ret i32 %10
}

define i32 @findFirstMultipleOf7() {
entry:
	%i = alloca i32
	%max = alloca i32
	store i32 0, i32* %max
	store i32 1, i32* %i
	br label %while.cond.9

while.cond.9:
	br i1 %2, label %while.body.10, label %while.end.11

while.body.10:
	br label %while.cond.9

while.end.11:
	%0 = load i32, i32* %i
	%1 = load i32, i32* %max
	%2 = icmp sle i32 %0, %1
	%3 = load i32, i32* %i
	%4 = srem i32 %3, 7
	%5 = icmp eq i32 %4, 0
	br i1 %5, label %if.then.12, label %if.merge.13

if.then.12:
	br label %if.merge.13

if.merge.13:
	%6 = load i32, i32* %i
	%7 = load i32, i32* %i
	%8 = load i32, i32* %i
	%9 = add i32 %8, 1
	ret i32 0
}

declare i32 @printf(i8*, ...)


; Standard format strings
@.fmt.int = private constant [4 x i8] c"%d\0A\00"
@.fmt.str = private constant [4 x i8] c"%s\0A\00"
@.fmt.float = private constant [4 x i8] c"%f\0A\00"
@.fmt.bool = private constant [4 x i8] c"%d\0A\00"
