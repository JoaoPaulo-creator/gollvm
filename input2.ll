
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

@.str.0 = global [21 x i8] c"this return a string\00"
@.str.1 = global [4 x i8] c"%d\0A\00"

















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
	ret i32 2
}

define i8* @baz() {
entry:
	ret i8* getelementptr ([21 x i8], [21 x i8]* @.str.0, i64 0, i64 0)
}

define i32 @main() {
entry:
	%0 = alloca i32
	%1 = alloca i32
	%2 = call i32 () @foo()
	%3 = call i32 () @foo()
	%4 = call i32 @printf(i8* getelementptr ([4 x i8], [4 x i8]* @.str.1, i64 0, i64 0), i32 0)
	%5 = call i32 () @bar()
	%6 = call i32 () @bar()
	%7 = call i32 @printf(i8* getelementptr ([4 x i8], [4 x i8]* @.str.1, i64 0, i64 0), i32 0)
	%8 = call i8* () @baz()
	%9 = call i8* () @baz()
	%10 = call i32 @printf(i8* getelementptr ([4 x i8], [4 x i8]* @.str.1, i64 0, i64 0), i32 0)
	store i32 5, i32* %1
	%11 = load i32, i32* %1
	%12 = mul i32 %11, 2
	store i32 %12, i32* %0
	%13 = load i32, i32* %0
	%14 = call i32 @printf(i8* getelementptr ([4 x i8], [4 x i8]* @.str.1, i64 0, i64 0), i32 %13)
	%15 = mul i32 2, 2
	%16 = mul i32 %15, 2
	%17 = call i32 @printf(i8* getelementptr ([4 x i8], [4 x i8]* @.str.1, i64 0, i64 0), i32 %16)
	ret i32 0
}
