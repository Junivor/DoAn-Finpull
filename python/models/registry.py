"""
Model Registry for versioning and metadata management.
Supports saving/loading model metadata, versioning, and model discovery.
"""
import json
import os
import shutil
from datetime import datetime
from typing import Any, Dict, List, Optional
from pathlib import Path


class ModelRegistry:
    """Registry for managing ML models with versioning."""
    
    def __init__(self, base_dir: str = "./python/models"):
        self.base_dir = Path(base_dir)
        self.base_dir.mkdir(parents=True, exist_ok=True)
    
    def _get_metadata_path(self, model_name: str, version: str) -> Path:
        """Get path to metadata file for a model version."""
        return self.base_dir / model_name / version / "metadata.json"
    
    def _get_model_dir(self, model_name: str, version: str) -> Path:
        """Get directory path for a model version."""
        return self.base_dir / model_name / version
    
    def save_model_metadata(
        self,
        model_name: str,
        version: str,
        metadata: Dict[str, Any],
        model_files: Optional[List[str]] = None
    ) -> Path:
        """
        Save model metadata with versioning.
        
        Args:
            model_name: Name of the model (e.g., "edge_score", "regime")
            version: Version string (e.g., "v1", "v1.0.0", "20241031")
            metadata: Dictionary of metadata (model_type, features, metrics, etc.)
            model_files: Optional list of model file names (e.g., ["model.joblib", "calib.joblib"])
        
        Returns:
            Path to saved metadata file
        """
        model_dir = self._get_model_dir(model_name, version)
        model_dir.mkdir(parents=True, exist_ok=True)
        
        # Enrich metadata
        full_metadata = {
            "model_name": model_name,
            "version": version,
            "created_at": datetime.utcnow().isoformat(),
            "model_files": model_files or [],
            **metadata
        }
        
        metadata_path = self._get_metadata_path(model_name, version)
        with open(metadata_path, "w") as f:
            json.dump(full_metadata, f, indent=2)
        
        return metadata_path
    
    def load_model_metadata(self, model_name: str, version: Optional[str] = None) -> Dict[str, Any]:
        """
        Load model metadata.
        
        Args:
            model_name: Name of the model
            version: Version string. If None, loads latest version.
        
        Returns:
            Metadata dictionary
        """
        if version is None:
            version = self.get_latest_version(model_name)
            if version is None:
                raise ValueError(f"No versions found for model '{model_name}'")
        
        metadata_path = self._get_metadata_path(model_name, version)
        
        if not metadata_path.exists():
            raise FileNotFoundError(
                f"Metadata not found for {model_name} version {version} at {metadata_path}"
            )
        
        with open(metadata_path, "r") as f:
            return json.load(f)
    
    def get_latest_version(self, model_name: str) -> Optional[str]:
        """
        Get the latest version of a model by creation timestamp.
        
        Args:
            model_name: Name of the model
        
        Returns:
            Latest version string or None if no versions exist
        """
        model_base = self.base_dir / model_name
        if not model_base.exists():
            return None
        
        versions = []
        for version_dir in model_base.iterdir():
            if not version_dir.is_dir():
                continue
            
            metadata_path = version_dir / "metadata.json"
            if metadata_path.exists():
                try:
                    with open(metadata_path, "r") as f:
                        meta = json.load(f)
                        created_at = meta.get("created_at", "")
                        versions.append((meta.get("version", version_dir.name), created_at))
                except Exception:
                    versions.append((version_dir.name, ""))
        
        if not versions:
            return None
        
        # Sort by created_at (most recent first)
        versions.sort(key=lambda x: x[1], reverse=True)
        return versions[0][0]
    
    def list_versions(self, model_name: str) -> List[Dict[str, Any]]:
        """
        List all versions of a model with metadata.
        
        Args:
            model_name: Name of the model
        
        Returns:
            List of version info dictionaries
        """
        model_base = self.base_dir / model_name
        if not model_base.exists():
            return []
        
        versions = []
        for version_dir in model_base.iterdir():
            if not version_dir.is_dir():
                continue
            
            metadata_path = version_dir / "metadata.json"
            if metadata_path.exists():
                try:
                    with open(metadata_path, "r") as f:
                        meta = json.load(f)
                        versions.append({
                            "version": meta.get("version", version_dir.name),
                            "created_at": meta.get("created_at", ""),
                            "model_type": meta.get("model_type", ""),
                            "metrics": meta.get("metrics", {})
                        })
                except Exception:
                    versions.append({
                        "version": version_dir.name,
                        "created_at": "",
                        "model_type": "",
                        "metrics": {}
                    })
        
        # Sort by created_at (most recent first)
        versions.sort(key=lambda x: x.get("created_at", ""), reverse=True)
        return versions
    
    def list_models(self) -> List[str]:
        """
        List all registered model names.
        
        Returns:
            List of model names
        """
        if not self.base_dir.exists():
            return []
        
        models = []
        for item in self.base_dir.iterdir():
            if item.is_dir() and not item.name.startswith("."):
                # Check if it has at least one version
                if any(v.is_dir() for v in item.iterdir()):
                    models.append(item.name)
        
        return sorted(models)
    
    def promote_version(self, model_name: str, from_version: str, to_version: str) -> bool:
        """
        Promote a model version (copy to new version).
        Useful for promoting staging → production.
        
        Args:
            model_name: Name of the model
            from_version: Source version
            to_version: Target version
        
        Returns:
            True if successful
        """
        from_dir = self._get_model_dir(model_name, from_version)
        to_dir = self._get_model_dir(model_name, to_version)
        
        if not from_dir.exists():
            return False
        
        # Copy all files
        to_dir.mkdir(parents=True, exist_ok=True)
        for item in from_dir.iterdir():
            if item.is_file():
                shutil.copy2(item, to_dir / item.name)
            elif item.is_dir():
                shutil.copytree(item, to_dir / item.name, dirs_exist_ok=True)
        
        # Update metadata
        try:
            meta = self.load_model_metadata(model_name, from_version)
            meta["version"] = to_version
            meta["promoted_from"] = from_version
            meta["promoted_at"] = datetime.utcnow().isoformat()
            
            self.save_model_metadata(model_name, to_version, meta)
            return True
        except Exception:
            return False
    
    def delete_version(self, model_name: str, version: str) -> bool:
        """
        Delete a model version.
        
        Args:
            model_name: Name of the model
            version: Version to delete
        
        Returns:
            True if successful
        """
        model_dir = self._get_model_dir(model_name, version)
        if model_dir.exists():
            shutil.rmtree(model_dir)
            return True
        return False


# Convenience functions (backward compatibility)
def save_metadata(path: str, meta: Dict[str, Any]) -> None:
    """Legacy function - save metadata to a specific path."""
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "w") as f:
        json.dump(meta, f, indent=2)


def load_metadata(path: str) -> Dict[str, Any]:
    """Legacy function - load metadata from a specific path."""
    with open(path, "r") as f:
        return json.load(f)
