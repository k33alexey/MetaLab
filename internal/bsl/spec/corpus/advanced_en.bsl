Procedure AdvancedStatements(Source, Handler)
    AddHandler Source.Event, Handler;
    Execute "Result = 1;";
    Goto ~Finish;
    Result = 2;
    ~Finish:
    RemoveHandler Source.Event, Handler;
EndProcedure
