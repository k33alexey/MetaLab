&Before("Write")
Procedure BeforeWrite()
#Insert
    Message("Extension");
#EndInsert
EndProcedure

&After("Write")
Procedure AfterWrite()
EndProcedure

&Around("Posting")
Procedure AroundPosting()
EndProcedure

&ChangeAndValidate("Validation")
Procedure ValidateChange()
EndProcedure

#Delete
Procedure Removed()
EndProcedure
#EndDelete
