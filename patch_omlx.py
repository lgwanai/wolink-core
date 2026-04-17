import os
import shutil

# Target paths for the installed macOS App bundle
OMLX_DIR = "/Applications/oMLX.app/Contents/Resources/omlx"
API_MODELS_FILE = os.path.join(OMLX_DIR, "api", "audio_models.py")
API_ROUTES_FILE = os.path.join(OMLX_DIR, "api", "audio_routes.py")
STT_ENGINE_FILE = os.path.join(OMLX_DIR, "engine", "stt.py")

def backup_file(file_path):
    if os.path.exists(file_path):
        backup_path = file_path + ".bak"
        if not os.path.exists(backup_path):
            shutil.copy2(file_path, backup_path)
            print(f"Backed up {file_path} to {backup_path}")

def patch_audio_models():
    print(f"Patching {API_MODELS_FILE}...")
    backup_file(API_MODELS_FILE)
    
    with open(API_MODELS_FILE, "r") as f:
        content = f.read()
        
    if "char_level_info" not in content:
        content = content.replace(
            "    segments: Optional[List[dict]] = None\n",
            "    segments: Optional[List[dict]] = None\n    char_level_info: Optional[List[dict]] = None\n"
        )
        
        with open(API_MODELS_FILE, "w") as f:
            f.write(content)
        print("✅ audio_models.py patched successfully.")
    else:
        print("⚠️ audio_models.py already contains char_level_info, skipping.")

def patch_audio_routes():
    print(f"Patching {API_ROUTES_FILE}...")
    backup_file(API_ROUTES_FILE)
    
    with open(API_ROUTES_FILE, "r") as f:
        content = f.read()
        
    # 1. Add Request to imports
    if "from fastapi import Request" not in content and "from fastapi import APIRouter, File, Form, HTTPException, UploadFile, Request" not in content:
        content = content.replace(
            "from fastapi import APIRouter, File, Form, HTTPException, UploadFile\n",
            "from fastapi import APIRouter, File, Form, HTTPException, UploadFile, Request\n"
        )
        
    # 2. Modify create_transcription signature and body
    if "request: Request" not in content:
        content = content.replace(
            "async def create_transcription(\n    file: UploadFile = File(...),",
            "async def create_transcription(\n    request: Request,\n    file: UploadFile = File(...),"
        )
        
        # Add logic to extract forced_aligner and pass it
        search_str = "pool = _get_engine_pool()"
        replace_str = """
    form_data = await request.form()
    forced_aligner = form_data.get("forced_aligner")
    
    pool = _get_engine_pool()"""
        content = content.replace(search_str, replace_str)
        
        # Pass to engine.transcribe
        content = content.replace(
            "result = await engine.transcribe(tmp_path, language=language)",
            "result = await engine.transcribe(tmp_path, language=language, forced_aligner=forced_aligner)"
        )
        
        # Add to AudioTranscriptionResponse
        content = content.replace(
            "segments=segments,\n    )",
            "segments=segments,\n        char_level_info=result.get(\"char_level_info\"),\n    )"
        )
        
        with open(API_ROUTES_FILE, "w") as f:
            f.write(content)
        print("✅ audio_routes.py patched successfully.")
    else:
        print("⚠️ audio_routes.py already patched, skipping.")

def patch_stt_engine():
    print(f"Patching {STT_ENGINE_FILE}...")
    backup_file(STT_ENGINE_FILE)
    
    with open(STT_ENGINE_FILE, "r") as f:
        content = f.read()
        
    # Add forced alignment logic to _transcribe_sync
    if "forced_aligner = kwargs.get(\"forced_aligner\")" not in content:
        search_str = """                return {
                    "text": result.text or "",
                    "language": raw_lang,
                    "segments": segments,
                    "duration": getattr(
                        result, "total_time", 0.0
                    ),
                }"""
                
        replace_str = """                response_dict = {
                    "text": result.text or "",
                    "language": raw_lang,
                    "segments": segments,
                    "duration": getattr(
                        result, "total_time", 0.0
                    ),
                }
                
                # --- ADDED: Forced Alignment Logic ---
                forced_aligner = kwargs.get("forced_aligner")
                if forced_aligner and response_dict["text"]:
                    try:
                        from mlx_audio.stt.utils import load_model as mlx_load_model
                        import os
                        
                        # Resolve the aligner model path (usually in ~/.omlx/models)
                        aligner_path = os.path.expanduser(f"~/.omlx/models/{forced_aligner}")
                        if not os.path.exists(aligner_path):
                            aligner_path = forced_aligner
                            
                        logger.info(f"Running forced alignment with {aligner_path}...")
                        aligner_model = mlx_load_model(aligner_path)
                        
                        align_lang = response_dict["language"]
                        if not align_lang or str(align_lang).lower() == "none":
                            align_lang = "English"
                            
                        align_res = aligner_model.generate(
                            audio=audio_path,
                            text=response_dict["text"],
                            language=align_lang
                        )
                        
                        char_info = []
                        if isinstance(align_res, list): 
                            align_res = align_res[0]
                            
                        if hasattr(align_res, "items"):
                            for item in align_res.items:
                                char_info.append({
                                    "text": item.text,
                                    "start": item.start_time,
                                    "end": item.end_time
                                })
                        response_dict["char_level_info"] = char_info
                        logger.info(f"Forced alignment complete, generated {len(char_info)} char-level timestamps.")
                    except Exception as e:
                        logger.error(f"Forced alignment failed: {e}")
                # -------------------------------------
                
                return response_dict"""
                
        content = content.replace(search_str, replace_str)
        
        with open(STT_ENGINE_FILE, "w") as f:
            f.write(content)
        print("✅ stt.py patched successfully.")
    else:
        print("⚠️ stt.py already patched, skipping.")

if __name__ == "__main__":
    print("🚀 Starting oMLX patch for char_level_info...")
    try:
        patch_audio_models()
        patch_audio_routes()
        patch_stt_engine()
        print("🎉 Patching completed! Please restart your omlx.server.")
    except Exception as e:
        print(f"❌ Patching failed: {e}")