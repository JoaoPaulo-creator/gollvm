@.str.0 = global [11 x i8] c"ola, mundo\00"
@.str.1 = global [4 x i8] c"%s\0A\00"

declare i8* @malloc(i64)

declare void @free(i8*)

declare i64 @strlen(i8*)

declare i8* @strcpy(i8*, i8*)

declare i32 @abs(i32)

declare double @pow(double, double)

declare void @exit(i32)

define i32 @main() {
entry:
	%0 = bitcast i8* getelementptr ([4 x i8], [4 x i8]* @.str.1, i64 0, i64 0) to i8*
	%1 = call i32 (i8*) (...) @printf(i8* %0, i8* getelementptr ([11 x i8], [11 x i8]* @.str.0, i64 0, i64 0))
	ret i32 0
}

declare i32 @printf(i8*, ...)


; Standard format strings
@.fmt.int = private constant [4 x i8] c"%d\0A\00"
@.fmt.str = private constant [4 x i8] c"%s\0A\00"
@.fmt.float = private constant [4 x i8] c"%f\0A\00"
@.fmt.bool = private constant [4 x i8] c"%d\0A\00"
