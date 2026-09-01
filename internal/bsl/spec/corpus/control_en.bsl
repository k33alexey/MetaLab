Procedure ControlFlow(Collection)
    While True Do
        Break;
    EndDo;

    For Index = 0 To 10 Do
        If Index = 5 Then
            Continue;
        ElsIf Index > 8 Then
            Break;
        Else
            Message(Index);
        EndIf;
    EndDo;

    For Each Item In Collection Do
        Try
            Process(Item);
        Except
            Raise ErrorDescription();
        EndTry;
    EndDo;
EndProcedure
