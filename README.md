# kotlin-package-manager

CLI de Go para agregar dependencias a proyectos Kotlin Toolchain.

```powershell
go build -o kt.exe .
```

Pon la carpeta de `kt.exe` en tu PATH o invoca el ejecutable por su ruta absoluta.
Desde la raíz de un proyecto con `project.yaml`, o desde una subcarpeta:

```powershell
kt add coil
kt add coil -m shared
kt add io.coil-kt.coil3:coil-compose:3.6.3 -m shared
```

El módulo predeterminado es `shared`. El alias `coil` agrega Coil Compose y el
transporte OkHttp para JVM/Android, ambos en la versión 3.6.3. La CLI evita
duplicados, rechaza conflictos de versión explícitos y ejecuta el wrapper
`kotlin build`. Si el build falla, conserva el cambio en `module.yaml` y devuelve
el código de salida del wrapper. Usa un proyecto Kotlin Toolchain con su wrapper.
