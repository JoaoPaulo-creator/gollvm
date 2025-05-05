
    ; Standard C library function declarations
    declare i32 @printf(i8*, ...)
    declare i8* @malloc(i64)
    declare void @free(i8*)
    declare i64 @strlen(i8*)
    declare i8* @strcpy(i8*, i8*)
    declare i32 @abs(i32)
    declare double @pow(double, double)
    declare void @exit(i32)
    @factorial = global i32 (i32) ()** @factorial
@fibonacci = global i32 (i32) ()** @fibonacci
@sum = global i32 (i32) ()** @sum
@findFirstMultipleOf7 = global i32 (i32) ()** @findFirstMultipleOf7
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
	br i1 %2, label %if.then.0, label %if.merge.1

if.then.0:
	br label %if.merge.1

if.merge.1:
	%3 = load i32, i32* %n
	%4 = load i32, i32* %n
	%5 = sub i32 %4, 1
	%6 = call i32 (i32) @factorial(i32 %5)
	%7 = mul i32 %3, %6
	ret i32 %7
}

define i32 @fibonacci(i32) {
entry:
	%n = alloca i32
	store i32 0, i32* %n
	%1 = load i32, i32* %n
	%2 = icmp sle i32 %1, 0
	br i1 %2, label %if.then.2, label %if.merge.3

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
	%11 = add i32 %7, %10
	ret i32 %11
}

define i32 @sum(i32) {
entry:
	%i = alloca i32
	%total = alloca i32
	%n = alloca i32
	store i32 0, i32* %n
	store i32 0, i32* %total
	store i32 1, i32* %i
	br label %while.cond.6

while.cond.6:
	br i1 %3, label %while.body.7, label %while.end.8

while.body.7:
	br label %while.cond.6

while.end.8:
	%1 = load i32, i32* %i
	%2 = load i32, i32* %n
	%3 = icmp sle i32 %2, %2
	%4 = load i32, i32* %total
	%5 = load i32, i32* %total
	%6 = load i32, i32* %i
	%7 = add i32 %6, %6
	%8 = load i32, i32* %i
	%9 = load i32, i32* %i
	%10 = add i32 %9, 1
	%11 = load i32, i32* %total
	ret i32 %11
}

define i32 @findFirstMultipleOf7(i32) {
entry:
	%i = alloca i32
	%max = alloca i32
	store i32 0, i32* %max
	store i32 1, i32* %i
	br label %while.cond.9

while.cond.9:
	br i1 %3, label %while.body.10, label %while.end.11

while.body.10:
	br label %while.cond.9

while.end.11:
	%1 = load i32, i32* %i
	%2 = load i32, i32* %max
	%3 = icmp sle i32 %2, %2
	%4 = load i32, i32* %i
	%5 = srem i32 %4, 7
	%6 = icmp eq i32 %5, 0
	br i1 %6, label %if.then.12, label %if.merge.13

if.then.12:
	br label %if.merge.13

if.merge.13:
	%7 = load i32, i32* %i
	%8 = load i32, i32* %i
	%9 = load i32, i32* %i
	%10 = add i32 %9, 1
	ret i32 0
}

define i32 @main() {
entry:
	%result = alloca i32
	%z = alloca i32
	%y = alloca i32
	%x = alloca i32
	store i32 5, i32* %x
	store i32 10, i32* %y
	%1 = load i32, i32* %x
	%2 = load i32, i32* %y
	%3 = add i32 %2, %2
	store i32 %3, i32* %z
	%4 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.1, i64 0, i64 0) to [4 x i8]*
	%5 = call i32 @printf([4 x i8]* %4, i8* getelementptr ([23 x i8], [23 x i8]* @.str.0, i64 0, i64 0))
	%6 = call i32 (i32) @factorial(i32 5)
	%7 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%8 = alloca i32 (i32)
	store i32 %6, i32 (i32)* %8
	%9 = call i32 @printf([4 x i8]* %8, i32 (i32)* %8)
	%10 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%11 = call i32 @printf([4 x i8]* %10, i8* getelementptr ([23 x i8], [23 x i8]* @.str.2, i64 0, i64 0))
	%12 = call i32 (i32) @fibonacci(i32 10)
	%13 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%14 = alloca i32 (i32)
	store i32 %12, i32 (i32)* %14
	%15 = call i32 @printf([4 x i8]* %14, i32 (i32)* %14)
	%16 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%17 = call i32 @printf([4 x i8]* %16, i8* getelementptr ([36 x i8], [36 x i8]* @.str.3, i64 0, i64 0))
	%18 = call i32 (i32) @sum(i32 100)
	%19 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%20 = alloca i32 (i32)
	store i32 %18, i32 (i32)* %20
	%21 = call i32 @printf([4 x i8]* %20, i32 (i32)* %20)
	%22 = load i32, i32* %x
	%23 = load i32, i32* %y
	%24 = add i32 %23, %23
	%25 = load i32, i32* %z
	%26 = sub i32 %25, 5
	%27 = mul i32 %24, %26
	store i32 %27, i32* %result
	%28 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%29 = call i32 @printf([4 x i8]* %28, i8* getelementptr ([27 x i8], [27 x i8]* @.str.4, i64 0, i64 0))
	%30 = load i32, i32* %result
	%31 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.5, i64 0, i64 0) to [4 x i8]*
	%32 = alloca i32
	store i32 %30, i32 %32
	%33 = call i32 @printf([4 x i8]* %32, i32* %32)
	%34 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%35 = call i32 @printf([4 x i8]* %34, i8* getelementptr ([14 x i8], [14 x i8]* @.str.6, i64 0, i64 0))
	%36 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%37 = call i32 @printf([4 x i8]* %36, i8* getelementptr ([20 x i8], [20 x i8]* @.str.7, i64 0, i64 0))
	%38 = load i32, i32* %x
	%39 = load i32, i32* %y
	%40 = icmp slt i32 %39, %39
	%41 = bitcast [4 x i8]* @.str.5 to [4 x i8]*
	%42 = alloca i1
	store i1 %40, i1* %42
	%43 = call i32 @printf([4 x i8]* %42, i1* %42)
	%44 = load i32, i32* %x
	%45 = load i32, i32* %y
	%46 = icmp sgt i32 %45, %45
	%47 = bitcast [4 x i8]* @.str.5 to [4 x i8]*
	%48 = alloca i1
	store i1 %46, i1* %48
	%49 = call i32 @printf([4 x i8]* %48, i1* %48)
	%50 = load i32, i32* %x
	%51 = icmp eq i32 %50, 5
	%52 = bitcast [4 x i8]* @.str.5 to [4 x i8]*
	%53 = alloca i1
	store i1 %51, i1* %53
	%54 = call i32 @printf([4 x i8]* %53, i1* %53)
	%55 = load i32, i32* %y
	%56 = icmp ne i32 %55, 10
	%57 = bitcast [4 x i8]* @.str.5 to [4 x i8]*
	%58 = alloca i1
	store i1 %56, i1* %58
	%59 = call i32 @printf([4 x i8]* %58, i1* %58)
	%60 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%61 = call i32 @printf([4 x i8]* %60, i8* getelementptr ([30 x i8], [30 x i8]* @.str.8, i64 0, i64 0))
	%62 = load i32, i32* %x
	%63 = icmp slt i32 %62, 10
	br i1 %63, label %if.then.14, label %if.else.16

if.then.14:
	br label %if.merge.15

if.merge.15:
	br label %if.else.16

if.else.16:
	%64 = load i32, i32* %y
	%65 = icmp sgt i32 %64, 5
	br i1 %65, label %if.then.17, label %if.else.19

if.then.17:
	br label %if.merge.18

if.merge.18:
	br label %if.else.19

if.else.19:
	%66 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.10, i64 0, i64 0) to [4 x i8]*
	%67 = call i32 @printf([4 x i8]* %66, i8* getelementptr ([25 x i8], [25 x i8]* @.str.9, i64 0, i64 0))
	%68 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.12, i64 0, i64 0) to [4 x i8]*
	%69 = call i32 @printf([4 x i8]* %68, i8* getelementptr ([29 x i8], [29 x i8]* @.str.11, i64 0, i64 0))
	%70 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.14, i64 0, i64 0) to [4 x i8]*
	%71 = call i32 @printf([4 x i8]* %70, i8* getelementptr ([25 x i8], [25 x i8]* @.str.13, i64 0, i64 0))
	%72 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%73 = call i32 @printf([4 x i8]* %72, i8* getelementptr ([21 x i8], [21 x i8]* @.str.15, i64 0, i64 0))
	%74 = call i32 (i32) @findFirstMultipleOf7(i32 20)
	%75 = bitcast [4 x i8]* @.str.1 to [4 x i8]*
	%76 = alloca i32 (i32)
	store i32 %74, i32 (i32)* %76
	%77 = call i32 @printf([4 x i8]* %76, i32 (i32)* %76)
	br label %if.merge.18
}


; Standard format strings
@.fmt.int = private constant [4 x i8] c"%d\0A\00"
@.fmt.str = private constant [4 x i8] c"%s\0A\00"
@.fmt.float = private constant [4 x i8] c"%f\0A\00"
@.fmt.bool = private constant [4 x i8] c"%d\0A\00"
