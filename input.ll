
; Standard C library function declarations
declare i32 @printf(i8*, ...)
declare i8* @malloc(i64)
declare void @free(i8*)
declare i64 @strlen(i8*)
declare i8* @strcpy(i8*, i8*)
declare i32 @abs(i32)
declare double @pow(double, double)
declare void @exit(i32)


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
  %x = alloca i32
  %y = alloca i32
  %z = alloca i32
  %result = alloca i32
  
  ; Initialize variables
  store i32 5, i32* %x
  store i32 10, i32* %y
  
  ; Calculate z = x + y
  %x.val = load i32, i32* %x
  %y.val = load i32, i32* %y
  %add = add i32 %x.val, %y.val
  store i32 %add, i32* %z
  
  ; Print "Factorial calculation:"
  %str1 = getelementptr [25 x i8], [25 x i8]* @.str.0, i32 0, i32 0
  call i32 (i8*, ...) @printf(i8* %str1)
  
  ; Print factorial(5)
  %fact = call i32 @factorial(i32 5)
  %str5 = getelementptr [7 x i8], [7 x i8]* @.str.5, i32 0, i32 0
  call i32 (i8*, ...) @printf(i8* %str5, i32 %fact)
  
  ; Print "Fibonacci calculation:"
  %str2 = getelementptr [25 x i8], [25 x i8]* @.str.2, i32 0, i32 0
  call i32 (i8*, ...) @printf(i8* %str2)
  
  ; Print fibonacci(10)
  %fib = call i32 @fibonacci(i32 10)
  call i32 (i8*, ...) @printf(i8* %str5, i32 %fib)
  
  ; Print "Sum calculation (using while loop):"
  %str3 = getelementptr [38 x i8], [38 x i8]* @.str.3, i32 0, i32 0
  call i32 (i8*, ...) @printf(i8* %str3)
  
  ; Print sum(100)
  %sum = call i32 @sum(i32 100)
  call i32 (i8*, ...) @printf(i8* %str5, i32 %sum)
  
  ; Calculate complex arithmetic
  %x.val2 = load i32, i32* %x
  %y.val2 = load i32, i32* %y
  %add2 = add i32 %x.val2, %y.val2
  %z.val = load i32, i32* %z
  %sub = sub i32 %z.val, 5
  %mul = mul i32 %add2, %sub
  store i32 %mul, i32* %result
  
  ; Print "Complex arithmetic result:"
  %str4 = getelementptr [29 x i8], [29 x i8]* @.str.4, i32 0, i32 0
  call i32 (i8*, ...) @printf(i8* %str4)
  
  ; Print result
  %result.val = load i32, i32* %result
  call i32 (i8*, ...) @printf(i8* %str5, i32 %result.val)
  
  ; Print "Hello, world!"
  %str6 = getelementptr [16 x i8], [16 x i8]* @.str.6, i32 0, i32 0
  call i32 (i8*, ...) @printf(i8* %str6)
  
  ; Print "First multiple of 7:"
  %str15 = getelementptr [23 x i8], [23 x i8]* @.str.15, i32 0, i32 0
  call i32 (i8*, ...) @printf(i8* %str15)
  
  ; Print findFirstMultipleOf7(20)
  %first7 = call i32 @findFirstMultipleOf7(i32 20)
  call i32 (i8*, ...) @printf(i8* %str5, i32 %first7)
  
  ret i32 0
}

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

















define void @global_init() {
entry:
	ret void
}










