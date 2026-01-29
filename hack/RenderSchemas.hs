{-# LANGUAGE OverloadedStrings #-}

module Main where

import Data.Char qualified as Char
import Data.Foldable (traverse_)
import Data.Map.Strict (Map)
import Data.Map.Strict qualified as Map
import Data.Text (Text)
import Data.Text qualified as Text
import Data.Text.IO qualified as Text
import Data.Yaml qualified as Yaml
import Dhall.Core (pretty)
import Dhall.Core qualified as Dhall
import Dhall.Kubernetes.Convert qualified as Convert
import Dhall.Kubernetes.Types (Definition (typ), Expr, ModelName (..))
import Dhall.Map qualified as DMap
import Lens.Micro qualified as Lens

main :: IO ()
main = do
    -- Read the CRD schemas
    crd <- Yaml.decodeFileThrow "config/crd/bases/sf.softwarefactory-project.io_softwarefactories.yaml"
    -- Convert into dhall schemas
    case Convert.toDefinition crd of
        Left err -> error $ "Couldn't parse the crd: " <> Text.unpack err
        Right result -> traverse_ writeDhall $ generateSchemas $ Map.fromList [result]
    putStrLn "Done!"
  where
    writeDhall (fp, content) = Text.writeFile ("schemas/" <> fp <> ".dhall") $ pretty content

-- | Generate one schema per model name, along with a global package.dhall import.
generateSchemas :: Map ModelName Definition -> [(FilePath, Expr)]
generateSchemas defs = [("package", package)] <> map toDhallFiles (Map.toList types)
  where
    package = getPackage $ Map.keys types
    types = getTypes defs
    defaults = getDefaults types
    toDhallFiles (model, typeExpr) =
        let defExpr = case Map.lookup model defaults of
                Just x -> x
                Nothing -> Dhall.RecordLit mempty
            schemaExpr =
                Dhall.RecordLit $
                    DMap.fromList
                        [ ("Type", Dhall.makeRecordField $ adjustImport typeExpr)
                        , ("default", Dhall.makeRecordField defExpr)
                        ]
         in (Text.unpack $ unModelName model, schemaExpr)

-- | Returns the Dhall type expr for the CRD.
getTypes :: Map ModelName Definition -> Map ModelName Expr
getTypes = fixModels . Convert.toTypes mempty splitModels True []
  where
    fixModels = Map.map fixEmptyType . Map.filterWithKey removeTop
    -- Remove the non-spec part of the CRD
    removeTop k _v = case unModelName k of
        "io.k8s.apimachinery.pkg.util.intstr.NatOrString" -> False
        "sf.softwarefactory-project.io.SoftwareFactory" -> False
        _ -> True

    -- Attribute like cpu, memory or storage size are undefined, this make them Text
    fixEmptyType :: Expr -> Expr
    fixEmptyType = Lens.transformOf Dhall.subExpressions emptyTypeToText
    emptyTypeToText expr = case expr of
        Dhall.Record m | m == mempty -> Dhall.Text
        _ -> expr

-- | Split the schemas into logical units
splitModels :: [ModelName] -> Definition -> Maybe ModelName
splitModels hierarchy def
    | -- We only split object schema, not values like array or strings
      typ def /= Just "object" =
        Nothing
    | otherwise = case hierarchy of
        -- The top level .spec attribute is moved into SoftwareFactorySpec
        [ModelName "sf.softwarefactory-project.io.SoftwareFactory", ModelName "spec"] ->
            Just $ ModelName "Spec"
        -- Adapt LogJuicer storage
        [ModelName "Spec", ModelName "logjuicer"] -> Just $ ModelName "Storage"
        -- Spec attributes are moved into dedicated models
        [ModelName "Spec", ModelName attr] ->
            Just $ ModelName $ adjustName attr
        -- Zuul and Nodepool attributes are moved into dedicated models
        [ModelName "Zuul", ModelName x]
            | "conns" `Text.isSuffixOf` x -> Just $ ModelName $ adjustName $ Text.replace "conns" "Conn" x
            | otherwise -> Just $ ModelName $ "Zuul" <> adjustName x
        [ModelName "Nodepool", ModelName x] ->
            Just $ ModelName $ "Nodepool" <> adjustName x
        -- Adapt MariaDB attributes
        [_, ModelName "logStorage"] -> Just $ ModelName "Storage"
        [_, ModelName "dbStorage"] -> Just $ ModelName "Storage"
        -- Move limits and storage attribute into dedicated models
        _ -> case last hierarchy of
            ModelName "limits" -> Just $ ModelName "Limits"
            ModelName "storage" -> Just $ ModelName "Storage"
            _ -> Nothing

-- | Convert model name to PascalCase
adjustName :: Text -> Text
adjustName = Text.filter (/= ' ') . Text.unwords . map toTitle . Text.words . Text.replace "-" " "
  where
    toTitle s = case Text.uncons s of
        Just (c, rest) -> Text.cons (Char.toUpper c) rest
        Nothing -> s

-- | Returns the Dhall default expr.
getDefaults :: Map ModelName Expr -> Map ModelName Expr
getDefaults = fmap adjustImport . Map.mapMaybeWithKey (Convert.toDefault mempty mempty)

{- | Adjust the import path because the default dhall-openapi converter generates multiple files
for the type and default while we want to keep them in a single schema file.
-}
adjustImport :: Expr -> Expr
adjustImport = Lens.transformOf Dhall.subExpressions toLocalType
  where
    toLocalType expr = case expr of
        Dhall.Embed (Dhall.Import (Dhall.ImportHashed _ (Dhall.Local _ (Dhall.File _ f))) _) ->
            Dhall.Field (mkImport f) $ Dhall.makeFieldSelection "Type"
        _ -> expr

mkImport :: Text -> Expr
mkImport = Dhall.Embed . Convert.mkImport mempty []

-- | Return a Dhall expression for the package
getPackage :: [ModelName] -> Expr
getPackage = Dhall.RecordLit . DMap.fromList . map toRecordField
  where
    toRecordField (ModelName name) = (name, Dhall.makeRecordField $ mkImport (name <> ".dhall"))
