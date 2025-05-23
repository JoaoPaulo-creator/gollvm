
; Standard function declarations
declare i32 @printf(i8*, ...)
declare i8* @malloc(i64)
declare void @free(i8*)
declare i64 @strlen(i8*)
declare i8* @strcpy(i8*, i8*)
declare i32 @abs(i32)
declare double @pow(double, double)
declare void @exit(i32)

; Standard format strings
@.fmt.int = private constant [4 x i8] c"%d\0A\00"
@.fmt.str = private constant [3 x i8] c"%s\00"

@.str.0 = global [34 x i8] c"a string returned from a function\00"
@.str.1 = global [3 x i8] c"%d\00"
@.str.2 = global [3 x i8] c"%s\00"

















define void @global_init() {
entry:
	ret void
}

define i32 @foo() {
entry:
	ret i32 4
}

define i32 @bar() {
entry:
	ret i32 42
}

define i8* @baz() {
entry:
	ret i8* getelementptr ([34 x i8], [34 x i8]* @.str.0, i64 0, i64 0)
}

define i32 @main() {
entry:
	%0 = alloca i32
	%1 = alloca i32
	%2 = call i32 () bitcast (i32 () ()* @foo to i32 () ()*)()
	%3 = bitcast i32 () %2 to i32 ()*
	%4 = call i32 %3()
	%5 = call i32 (i8*) (...) bitcast (i32 (i8*) (...)* @printf to i32 (i8*) (...)*)(i8* getelementptr ([3 x i8], [3 x i8]* @.str.1, i64 0, i64 0), i32 %4)
	%6 = call i32 () bitcast (i32 () ()* @bar to i32 () ()*)()
	%7 = bitcast i32 () %6 to i32 ()*
	%8 = call i32 %7()
	%9 = call i32 (i8*) (...) bitcast (i32 (i8*) (...)* @printf to i32 (i8*) (...)*)(i8* getelementptr ([3 x i8], [3 x i8]* @.str.1, i64 0, i64 0), i32 %8)
	%10 = call i8* () bitcast (i8* () ()* @baz to i8* () ()*)()
	%11 = bitcast i8* () %10 to i8* ()*
	%12 = call i8* %11()
	%13 = call i32 (i8*) (...) bitcast (i32 (i8*) (...)* @printf to i32 (i8*) (...)*)(i8* getelementptr ([3 x i8], [3 x i8]* @.str.2, i64 0, i64 0), i8* %12)
	store i32 5, i32* %1
	%14 = load i32, i32* %1
	%15 = mul i32 %14, 2
	store i32 %15, i32* %0
	%16 = load i32, i32* %0
	%17 = call i32 (i8*) (...) bitcast (i32 (i8*) (...)* @printf to i32 (i8*) (...)*)(i8* getelementptr ([3 x i8], [3 x i8]* @.str.1, i64 0, i64 0), i32 %16)
	%18 = mul i32 2, 2
	%19 = mul i32 %18, 2
	%20 = call i32 (i8*) (...) bitcast (i32 (i8*) (...)* @printf to i32 (i8*) (...)*)(i8* getelementptr ([3 x i8], [3 x i8]* @.str.1, i64 0, i64 0), i32 %19)
	ret i32 0
}
