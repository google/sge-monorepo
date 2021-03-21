shader packer tool to parse & compile shaders to spirv
Need to open proj with vs2019

each shader file could contain multiple entry points, make sure to declare all entrypoints by adding 
@shader([entrypointname], [shadertype]) 
in the shader file.

cmd: shaderpacker [shaderfilename]

Exp: shaderpacker shaders\lighting.hlsl