{ Type =
    { className : Optional Text
    , extraAnnotations : Optional (List { mapKey : Text, mapValue : Text })
    }
, default =
  { className = None Text
  , extraAnnotations = None (List { mapKey : Text, mapValue : Text })
  }
}