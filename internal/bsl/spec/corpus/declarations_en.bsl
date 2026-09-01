Var GlobalValue Export;

&AtServer
Procedure PerformAction(Val Parameter, Optional = Undefined) Export
    Var LocalValue;
    If Optional = Undefined Then
        Return;
    EndIf;
EndProcedure

&AtClient
Function GetValue() Export
    Return GlobalValue;
EndFunction

&AtServerNoContext
Procedure ServerWithoutContext()
EndProcedure

&AtClientAtServer
Procedure ClientAndServer()
EndProcedure

&AtClientAtServerNoContext
Procedure UniversalWithoutContext()
EndProcedure
