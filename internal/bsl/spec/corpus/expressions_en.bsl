Procedure Expressions(Object, Array)
    Result = New Structure("Key", 1);
    Another = New("Array");
    Value = Object.Method(1, , 3).Property[0];
    Array[0] = Value;
    Condition = Not (True And False) Or Undefined = Null;
    NumberValue = -(1 + 2) * 3 / 4 % 2;
    Choice = ?(Condition, "Yes", "No");
EndProcedure
