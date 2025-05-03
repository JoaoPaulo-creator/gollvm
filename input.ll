
	; Standard C library function declarations
	declare i32 @printf(i8*, ...)
	declare i8* @malloc(i64)
	declare void @free(i8*)
	declare i64 @strlen(i8*)
	declare i8* @strcpy(i8*, i8*)
	declare i32 @abs(i32)
	declare double @pow(double, double)
	declare void @exit(i32)
	@.str.0 = global [23 x i8] c"Factorial calculation:\00"
@.str.1 = global [4 x i8] c"%s\0A\00"
@.str.2 = global [23 x i8] c"Fibonacci calculation:\00"
@.str.3 = global [36 x i8] c"Sum calculation (using while loop):\00"
@.str.4 = global [27 x i8] c"Complex arithmetic result:\00"
@.str.5 = global [4 x i8] c"%d\0A\00"
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









define i32 @factorial(i32) {
entry:
	%n = alloca i32
	store i32 0, i32* %n
	%1 = load i32, i32* %n
	%2 = icmp sle i32 %1, 1
	br i1 %7, label %if.then.0, label %if.merge.1

if.then.0:
	br label %if.merge.1

if.merge.1:
	%3 = load i32, i32* %n
	%4 = load i32, i32* %n
	%5 = sub i32 %4, 1
	%6 = call i32 (i32) @factorial(i32 %5)
	%7 = mul i32 %4, %6
	ret i32 %7
}

define i32 @fibonacci(i32) {
entry:
	%n = alloca i32
	store i32 0, i32* %n
	%1 = load i32, i32* %n
	%2 = icmp sle i32 %1, 0
	br i1 %4, label %if.then.2, label %if.merge.3

if.then.2:
	br label %if.merge.3

if.merge.3:
	%3 = load i32, i32* %n
	%4 = icmp eq i32 %3, 1
	br i1 %4, label %if.then.4, label %if.merge.5

if.then.4:
	br label %if.merge.5

if.merge.5:
	%5 = load i32, i32* %n
	%6 = sub i32 %5, 1
	%7 = call i32 (i32) @fibonacci(i32 %6)
	%8 = load i32, i32* %n
	%9 = sub i32 %8, 2
	%10 = call i32 (i32) @fibonacci(i32 %9)
	%11 = add i32 %9, %10
	ret i32 %11
}

define i32 @sum(i32) {
entry:
	%1 = alloca i32
	%2 = alloca i32
	%n = alloca i32
	store i32 0, i32* %n
	store i32 0, i32* %2
	store i32 1, i32* %1
	br label %while.cond.6

while.cond.6:
	br i1 %3, label %while.body.7, label %while.end.8

while.body.7:
	br label %while.cond.6

while.end.8:
	%4 = load i32, i32* %1
	%5 = load i32, i32* %n
	%3 = icmp sle i32 %4, %7
	%6 = load i32, i32* %2
	%7 = load i32, i32* %4
	%8 = load i32, i32* %1
	%9 = add i32 %7, %8
	%10 = load i32, i32* %2
	%11 = load i32, i32* %1
	%12 = add i32 %11, 1
	%13 = load i32, i32* %4
	ret i32 %13
}

define i32 @findFirstMultipleOf7(i32) {
entry:
	%1 = alloca i32
	%max = alloca i32
	store i32 0, i32* %max
	store i32 1, i32* %3
	br label %while.cond.9

while.cond.9:
	br i1 %5, label %while.body.10, label %while.end.11

while.body.10:
	br label %while.cond.9

while.end.11:
	%3 = load i32, i32* %1
	%4 = load i32, i32* %max
	%2 = icmp sle i32 %3, %4
	%5 = load i32, i32* %3
	%6 = srem i32 %5, 7
	%7 = icmp eq i32 %6, 0
	br i1 %11, label %if.then.12, label %if.merge.13

if.then.12:
	br label %if.merge.13

if.merge.13:
	%8 = load i32, i32* %1
	%9 = load i32, i32* %3
	%10 = load i32, i32* %3
	%11 = add i32 %10, 1
	ret i32 0
}

define i32 () @main() {
entry:
	%1 = alloca i32
	%2 = alloca i32
	%3 = alloca i32
	%4 = alloca i32
	store i32 5, i32* %4
	store i32 10, i32* %4
	%5 = load i32, i32* %4
	%6 = load i32, i32* %4
	%7 = add i32 %6, %6
	store i32 %7, i32* %4
	%8 = call i32 (i8*) (...) @printf(i8* getelementptr ([4 x i8], [4 x i8]* @.str.1, i64 0, i64 0), i8* getelementptr ([23 x i8], [23 x i8]* @.str.0, i64 0, i64 0))
	%9 = call i32 (i32) @factorial(i32 5)
	%10 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i32 (i32) %9)
	%11 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i8* getelementptr ([23 x i8], [23 x i8]* @.str.2, i64 0, i64 0))
	%12 = call i32 (i32) @fibonacci(i32 10)
	%13 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i32 (i32) %12)
	%14 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i8* getelementptr ([36 x i8], [36 x i8]* @.str.3, i64 0, i64 0))
	%15 = call i32 (i32) @sum(i32 100)
	%16 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i32 (i32) %15)
	%17 = load i32, i32* %5
	%18 = load i32, i32* %4
	%19 = add i32 %18, %18
	%20 = load i32, i32* %4
	%21 = sub i32 %20, 5
	%22 = mul i32 %21, %21
	store i32 %23, i32* %1
	%23 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i8* getelementptr ([27 x i8], [27 x i8]* @.str.4, i64 0, i64 0))
	%24 = load i32, i32* %1
	%25 = call i32 (i8*) (...) @printf(i8* getelementptr ([4 x i8], [4 x i8]* @.str.5, i64 0, i64 0), i32 %24)
	%26 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i8* getelementptr ([14 x i8], [14 x i8]* @.str.6, i64 0, i64 0))
	%27 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i8* getelementptr ([20 x i8], [20 x i8]* @.str.7, i64 0, i64 0))
	%28 = load i32, i32* %4
	%29 = load i32, i32* %4
	%30 = icmp slt i32 %28, %29
	%31 = call i32 (i8*) (...) @printf([4 x i8]* @.str.5, i1 %30)
	%32 = load i32, i32* %5
	%33 = load i32, i32* %4
	%34 = icmp sgt i32 %32, %33
	%35 = call i32 (i8*) (...) @printf([4 x i8]* @.str.5, i1 %34)
	%36 = load i32, i32* %4
	%37 = icmp eq i32 %36, 5
	%38 = call i32 (i8*) (...) @printf([4 x i8]* @.str.5, i1 %37)
	%39 = load i32, i32* %4
	%40 = icmp ne i32 %39, 10
	%41 = call i32 (i8*) (...) @printf([4 x i8]* @.str.5, i1 %40)
	%42 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i8* getelementptr ([30 x i8], [30 x i8]* @.str.8, i64 0, i64 0))
	%43 = load i32, i32* %4
	%44 = icmp slt i32 %43, 10
	br i1 %44, label %if.then.14, label %if.else.16

if.then.14:
	br label %if.merge.15

if.merge.15:
	br label %if.else.16

if.else.16:
	%45 = load i32, i32* %4
	%46 = icmp sgt i32 %45, 5
	br i1 %46, label %if.then.17, label %if.else.19

if.then.17:
	br label %if.merge.18

if.merge.18:
	br label %if.else.19

if.else.19:
	%47 = call i32 (i8*) (...) @printf(i8* getelementptr ([4 x i8], [4 x i8]* @.str.10, i64 0, i64 0), i8* getelementptr ([25 x i8], [25 x i8]* @.str.9, i64 0, i64 0))
	%48 = call i32 (i8*) (...) @printf(i8* getelementptr ([4 x i8], [4 x i8]* @.str.12, i64 0, i64 0), i8* getelementptr ([29 x i8], [29 x i8]* @.str.11, i64 0, i64 0))
	%49 = call i32 (i8*) (...) @printf(i8* getelementptr ([4 x i8], [4 x i8]* @.str.14, i64 0, i64 0), i8* getelementptr ([25 x i8], [25 x i8]* @.str.13, i64 0, i64 0))
	%50 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i8* getelementptr ([21 x i8], [21 x i8]* @.str.15, i64 0, i64 0))
	%51 = call i32 (i32) @findFirstMultipleOf7(i32 20)
	%52 = call i32 (i8*) (...) @printf([4 x i8]* @.str.1, i32 (i32) %51)
	br label %if.merge.18
}


; Standard format strings
@.fmt.int = private constant [4 x i8] c"%d\0A\00"
@.fmt.str = private constant [4 x i8] c"%s\0A\00"
@.fmt.float = private constant [4 x i8] c"%f\0A\00"
@.fmt.bool = private constant [4 x i8] c"%d\0A\00"
