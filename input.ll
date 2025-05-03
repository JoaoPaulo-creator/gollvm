@.str.0 = global [25 x i8] c"Factorial calculation:\5C00"
@.str.1 = global [7 x i8] c"%s\5Cn\5C00"
@.str.2 = global [25 x i8] c"Fibonacci calculation:\5C00"
@.str.3 = global [38 x i8] c"Sum calculation (using while loop):\5C00"
@.str.4 = global [29 x i8] c"Complex arithmetic result:\5C00"
@.str.5 = global [7 x i8] c"%d\5Cn\5C00"
@.str.6 = global [16 x i8] c"Hello, world!\5C00"
@.str.7 = global [22 x i8] c"Comparison results:\5C00"
@.str.8 = global [32 x i8] c"Nested if-else demonstration:\5C00"
@.str.9 = global [27 x i8] c"Both conditions are true\5C00"
@.str.10 = global [7 x i8] c"%s\5Cn\5C00"
@.str.11 = global [31 x i8] c"Only first condition is true\5C00"
@.str.12 = global [7 x i8] c"%s\5Cn\5C00"
@.str.13 = global [27 x i8] c"First condition is false\5C00"
@.str.14 = global [7 x i8] c"%s\5Cn\5C00"
@.str.15 = global [23 x i8] c"First multiple of 7:\5C00"

declare i32 (i8*) @printf(...)

declare i8* (i64) @malloc()

declare void (i8*) @free()

declare i64 (i8*) @strlen()

declare i8* (i8*, i8*) @strcpy()

declare i32 (i32) @abs()

declare double (double, double) @pow()

declare void (i32) @exit()

define void @global_init() {
entry:
	ret void
}

define i32 (i32) @factorial() {
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
	%5 = call i32 (i32) @factorial(i32 %4)
	%6 = mul i32 %2, %5
	ret i32 %6
}

define i32 (i32) @fibonacci() {
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
	%6 = call i32 (i32) @fibonacci(i32 %5)
	%7 = load i32, i32* %n
	%8 = sub i32 %7, 2
	%9 = call i32 (i32) @fibonacci(i32 %8)
	%10 = add i32 (i32) %6, %9
	ret i32 (i32) %10
}

define i32 (i32) @sum() {
entry:
	%0 = alloca i32
	%1 = alloca i32
	%n = alloca i32
	store i32 0, i32* %n
	store i32 0, i32* %1
	store i32 1, i32* %0
	br label %while.cond.6

while.cond.6:
	br i1 %4, label %while.body.7, label %while.end.8

while.body.7:
	br label %while.cond.6

while.end.8:
	%2 = load i32, i32* %0
	%3 = load i32, i32* %n
	%4 = icmp sle i32 %2, %3
	%5 = load i32, i32* %1
	%6 = load i32, i32* %1
	%7 = load i32, i32* %0
	%8 = add i32 %6, %7
	%9 = load i32, i32* %0
	%10 = load i32, i32* %0
	%11 = add i32 %10, 1
	%12 = load i32, i32* %1
	ret i32 %12
}

define i32 (i32) @findFirstMultipleOf7() {
entry:
	%0 = alloca i32
	%max = alloca i32
	store i32 0, i32* %max
	store i32 1, i32* %0
	br label %while.cond.9

while.cond.9:
	br i1 %3, label %while.body.10, label %while.end.11

while.body.10:
	br label %while.cond.9

while.end.11:
	%1 = load i32, i32* %0
	%2 = load i32, i32* %max
	%3 = icmp sle i32 %1, %2
	%4 = load i32, i32* %0
	%5 = srem i32 %4, 7
	%6 = icmp eq i32 %5, 0
	br i1 %6, label %if.then.12, label %if.merge.13

if.then.12:
	br label %if.merge.13

if.merge.13:
	%7 = load i32, i32* %0
	%8 = load i32, i32* %0
	%9 = load i32, i32* %0
	%10 = add i32 %9, 1
	ret i32 0
}

define i32 () @main() {
entry:
	%0 = alloca i32
	%1 = alloca i32
	%2 = alloca i32
	%3 = alloca i32
	store i32 5, i32* %3
	store i32 10, i32* %2
	%4 = load i32, i32* %3
	%5 = load i32, i32* %2
	%6 = add i32 %4, %5
	store i32 %6, i32* %1
	%7 = call i32 (i8*) (...) @printf(i8* getelementptr ([5 x i8], [7 x i8]* @.str.1, i64 0, i64 0), i8* getelementptr ([23 x i8], [25 x i8]* @.str.0, i64 0, i64 0))
	%8 = call i32 (i32) @factorial(i32 5)
	%9 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1)
	%10 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1, i8* getelementptr ([23 x i8], [25 x i8]* @.str.2, i64 0, i64 0))
	%11 = call i32 (i32) @fibonacci(i32 10)
	%12 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1)
	%13 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1, i8* getelementptr ([36 x i8], [38 x i8]* @.str.3, i64 0, i64 0))
	%14 = call i32 (i32) @sum(i32 100)
	%15 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1)
	%16 = load i32, i32* %3
	%17 = load i32, i32* %2
	%18 = add i32 %16, %17
	%19 = load i32, i32* %1
	%20 = sub i32 %19, 5
	%21 = mul i32 %18, %20
	store i32 %21, i32* %0
	%22 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1, i8* getelementptr ([27 x i8], [29 x i8]* @.str.4, i64 0, i64 0))
	%23 = load i32, i32* %0
	%24 = call i32 (i8*) (...) @printf(i8* getelementptr ([5 x i8], [7 x i8]* @.str.5, i64 0, i64 0), i32 %23)
	%25 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1, i8* getelementptr ([14 x i8], [16 x i8]* @.str.6, i64 0, i64 0))
	%26 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1, i8* getelementptr ([20 x i8], [22 x i8]* @.str.7, i64 0, i64 0))
	%27 = load i32, i32* %3
	%28 = load i32, i32* %2
	%29 = icmp slt i32 %27, %28
	%30 = call i32 (i8*) (...) @printf([7 x i8]* @.str.5, i1 %29)
	%31 = load i32, i32* %3
	%32 = load i32, i32* %2
	%33 = icmp sgt i32 %31, %32
	%34 = call i32 (i8*) (...) @printf([7 x i8]* @.str.5, i1 %33)
	%35 = load i32, i32* %3
	%36 = icmp eq i32 %35, 5
	%37 = call i32 (i8*) (...) @printf([7 x i8]* @.str.5, i1 %36)
	%38 = load i32, i32* %2
	%39 = icmp ne i32 %38, 10
	%40 = call i32 (i8*) (...) @printf([7 x i8]* @.str.5, i1 %39)
	%41 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1, i8* getelementptr ([30 x i8], [32 x i8]* @.str.8, i64 0, i64 0))
	%42 = load i32, i32* %3
	%43 = icmp slt i32 %42, 10
	br i1 %43, label %if.then.14, label %if.else.16

if.then.14:
	br label %if.merge.15

if.merge.15:
	br label %if.else.16

if.else.16:
	%44 = load i32, i32* %2
	%45 = icmp sgt i32 %44, 5
	br i1 %45, label %if.then.17, label %if.else.19

if.then.17:
	br label %if.merge.18

if.merge.18:
	br label %if.else.19

if.else.19:
	%46 = call i32 (i8*) (...) @printf(i8* getelementptr ([5 x i8], [7 x i8]* @.str.10, i64 0, i64 0), i8* getelementptr ([25 x i8], [27 x i8]* @.str.9, i64 0, i64 0))
	%47 = call i32 (i8*) (...) @printf(i8* getelementptr ([5 x i8], [7 x i8]* @.str.12, i64 0, i64 0), i8* getelementptr ([29 x i8], [31 x i8]* @.str.11, i64 0, i64 0))
	%48 = call i32 (i8*) (...) @printf(i8* getelementptr ([5 x i8], [7 x i8]* @.str.14, i64 0, i64 0), i8* getelementptr ([25 x i8], [27 x i8]* @.str.13, i64 0, i64 0))
	%49 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1, i8* getelementptr ([21 x i8], [23 x i8]* @.str.15, i64 0, i64 0))
	%50 = call i32 (i32) @findFirstMultipleOf7(i32 20)
	%51 = call i32 (i8*) (...) @printf([7 x i8]* @.str.1)
	br label %if.merge.18
}
