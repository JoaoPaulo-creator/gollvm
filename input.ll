
; Standard C library function declarations
declare i32 @printf(i8*, ...)
declare i8* @malloc(i64)
declare void @free(i8*)
declare i64 @strlen(i8*)
declare i8* @strcpy(i8*, i8*)
declare i32 @abs(i32)
declare double @pow(double, double)
declare void @exit(i32)

; Format strings for clean output
@.fmt.int = private constant [4 x i8] c"%d\0A\00"

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


define i32 @factorial(i32 %n) {
entry:
  %cmp = icmp sle i32 %n, 1
  br i1 %cmp, label %if.then, label %if.else

if.then:
  ret i32 1

if.else:
  %sub = sub i32 %n, 1
  %call = call i32 @factorial(i32 %sub)
  %mul = mul i32 %n, %call
  ret i32 %mul
}

define i32 @fibonacci(i32 %n) {
entry:
  %cmp = icmp sle i32 %n, 0
  br i1 %cmp, label %if.then, label %if.else

if.then:
  ret i32 0

if.else:
  %cmp1 = icmp eq i32 %n, 1
  br i1 %cmp1, label %if.then1, label %if.else1

if.then1:
  ret i32 1

if.else1:
  %sub = sub i32 %n, 1
  %call = call i32 @fibonacci(i32 %sub)
  %sub1 = sub i32 %n, 2
  %call1 = call i32 @fibonacci(i32 %sub1)
  %add = add i32 %call, %call1
  ret i32 %add
}

define i32 @sum(i32 %n) {
entry:
  %total = alloca i32
  %i = alloca i32
  store i32 0, i32* %total
  store i32 1, i32* %i
  br label %while.cond

while.cond:
  %i.val = load i32, i32* %i
  %cmp = icmp sle i32 %i.val, %n
  br i1 %cmp, label %while.body, label %while.end

while.body:
  %total.val = load i32, i32* %total
  %i.val1 = load i32, i32* %i
  %add = add i32 %total.val, %i.val1
  store i32 %add, i32* %total
  %i.val2 = load i32, i32* %i
  %inc = add i32 %i.val2, 1
  store i32 %inc, i32* %i
  br label %while.cond

while.end:
  %total.val1 = load i32, i32* %total
  ret i32 %total.val1
}

define i32 @findFirstMultipleOf7(i32 %max) {
entry:
  %i = alloca i32
  store i32 1, i32* %i
  br label %while.cond

while.cond:
  %i.val = load i32, i32* %i
  %cmp = icmp sle i32 %i.val, %max
  br i1 %cmp, label %while.body, label %while.end

while.body:
  %i.val1 = load i32, i32* %i
  %rem = srem i32 %i.val1, 7
  %cmp1 = icmp eq i32 %rem, 0
  br i1 %cmp1, label %if.then, label %if.end

if.then:
  %i.val2 = load i32, i32* %i
  ret i32 %i.val2

if.end:
  %i.val3 = load i32, i32* %i
  %inc = add i32 %i.val3, 1
  store i32 %inc, i32* %i
  br label %while.cond

while.end:
  ret i32 0
}

define i32 @main() {
entry:
  ; Allocate local variables
  %1 = alloca i32
  %2 = alloca i32
  %3 = alloca i32
  %4 = alloca i32
  
  ; Initialize variables
  store i32 5, i32* %1
  store i32 10, i32* %2
  
  ; Calculate z = x + y
  %5 = load i32, i32* %1
  %6 = load i32, i32* %2
  %7 = add i32 %5, %6
  store i32 %7, i32* %3
  
  ; Print "Factorial calculation:" with a newline
  %8 = getelementptr [25 x i8], [25 x i8]* @.str.0, i32 0, i32 0
  %9 = call i32 (i8*, ...) @printf(i8* %8)
  
  ; Print factorial(5) with a newline
  %10 = call i32 @factorial(i32 5)
  %11 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.fmt.int, i32 0, i32 0), i32 %10)
  
  ; Print "Fibonacci calculation:" with a newline
  %13 = getelementptr [25 x i8], [25 x i8]* @.str.2, i32 0, i32 0
  %14 = call i32 (i8*, ...) @printf(i8* %13)
  
  ; Print fibonacci(10) with a newline
  %15 = call i32 @fibonacci(i32 10)
  %16 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.fmt.int, i32 0, i32 0), i32 %15)
  
  ; Print "Sum calculation (using while loop):" with a newline
  %18 = getelementptr [38 x i8], [38 x i8]* @.str.3, i32 0, i32 0
  %19 = call i32 (i8*, ...) @printf(i8* %18)
  
  ; Print sum(100) with a newline
  %20 = call i32 @sum(i32 100)
  %21 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.fmt.int, i32 0, i32 0), i32 %20)
  
  ; Calculate complex arithmetic
  %23 = load i32, i32* %1
  %24 = load i32, i32* %2
  %25 = add i32 %23, %24
  %26 = load i32, i32* %3
  %27 = sub i32 %26, 5
  %28 = mul i32 %25, %27
  store i32 %28, i32* %4
  
  ; Print "Complex arithmetic result:" with a newline
  %29 = getelementptr [29 x i8], [29 x i8]* @.str.4, i32 0, i32 0
  %30 = call i32 (i8*, ...) @printf(i8* %29)
  
  ; Print result with a newline
  %31 = load i32, i32* %4
  %32 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.fmt.int, i32 0, i32 0), i32 %31)
  
  ; Print "Hello, world!" with a newline
  %34 = getelementptr [16 x i8], [16 x i8]* @.str.6, i32 0, i32 0
  %35 = call i32 (i8*, ...) @printf(i8* %34)
  
  ; Print "Comparison results:" with a newline
  %36 = getelementptr [22 x i8], [22 x i8]* @.str.7, i32 0, i32 0
  %37 = call i32 (i8*, ...) @printf(i8* %36)
  
  ; Print x < y with a newline
  %38 = load i32, i32* %1
  %39 = load i32, i32* %2
  %40 = icmp slt i32 %38, %39
  %41 = zext i1 %40 to i32
  %42 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.fmt.int, i32 0, i32 0), i32 %41)
  
  ; Print x > y with a newline
  %44 = load i32, i32* %1
  %45 = load i32, i32* %2
  %46 = icmp sgt i32 %44, %45
  %47 = zext i1 %46 to i32
  %48 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.fmt.int, i32 0, i32 0), i32 %47)
  
  ; Print x == 5 with a newline
  %50 = load i32, i32* %1
  %51 = icmp eq i32 %50, 5
  %52 = zext i1 %51 to i32
  %53 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.fmt.int, i32 0, i32 0), i32 %52)
  
  ; Print y != 10 with a newline
  %55 = load i32, i32* %2
  %56 = icmp ne i32 %55, 10
  %57 = zext i1 %56 to i32
  %58 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.fmt.int, i32 0, i32 0), i32 %57)
  
  ; Print "Nested if-else demonstration:" with a newline
  %60 = getelementptr [32 x i8], [32 x i8]* @.str.8, i32 0, i32 0
  %61 = call i32 (i8*, ...) @printf(i8* %60)
  
  ; if (x < 10) { ... } else { ... }
  %62 = load i32, i32* %1
  %63 = icmp slt i32 %62, 10
  br i1 %63, label %if.then, label %if.else
  
if.then:
  ; if (y > 5) { ... } else { ... }
  %64 = load i32, i32* %2
  %65 = icmp sgt i32 %64, 5
  br i1 %65, label %if.then.inner, label %if.else.inner
  
if.then.inner:
  ; Print "Both conditions are true" with a newline
  %66 = getelementptr [27 x i8], [27 x i8]* @.str.9, i32 0, i32 0
  %67 = call i32 (i8*, ...) @printf(i8* %66)
  br label %if.end
  
if.else.inner:
  ; Print "Only first condition is true" with a newline
  %68 = getelementptr [31 x i8], [31 x i8]* @.str.11, i32 0, i32 0
  %69 = call i32 (i8*, ...) @printf(i8* %68)
  br label %if.end
  
if.else:
  ; Print "First condition is false" with a newline
  %70 = getelementptr [27 x i8], [27 x i8]* @.str.13, i32 0, i32 0
  %71 = call i32 (i8*, ...) @printf(i8* %70)
  br label %if.end
  
if.end:
  ; Print "First multiple of 7:" with a newline
  %72 = getelementptr [23 x i8], [23 x i8]* @.str.15, i32 0, i32 0
  %73 = call i32 (i8*, ...) @printf(i8* %72)
  
  ; Print findFirstMultipleOf7(20) with a newline
  %74 = call i32 @findFirstMultipleOf7(i32 20)
  %75 = call i32 (i8*, ...) @printf(i8* getelementptr inbounds ([4 x i8], [4 x i8]* @.fmt.int, i32 0, i32 0), i32 %74)
  
  ret i32 0
}
