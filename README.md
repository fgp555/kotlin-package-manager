# KPM · Kotlin Package Manager

CLI de Go para agregar dependencias a proyectos Kotlin Toolchain y Gradle Kotlin DSL.

```powershell
go install .
kpm version
```

Go instala `kpm.exe` en `GOBIN` o, si no está configurado, en `GOPATH/bin`. Agrega esa carpeta a tu PATH.
Desde un proyecto con `project.yaml` o `settings.gradle.kts`:

```powershell
kpm add coil-compose
kpm add coil-compose coil-network-okhttp -m shared
kpm add io.coil-kt.coil3:coil-compose
kpm add io.coil-kt.coil3:coil-compose:3.6.3
```

KPM usa el módulo desde el que ejecutas el comando. Desde la raíz, prefiere
`shared` y pide elegir si hay varios módulos candidatos. Usa `-m` para elegir
un módulo explícitamente.

En Kotlin Toolchain escribe en `module.yaml`. En Gradle escribe en
`build.gradle.kts`, en `commonMain.dependencies` o en un único bloque
`dependencies` de nivel superior. Usa `--source-set jvmMain` para elegir
un bloque existente `jvmMain.dependencies`. El editor admite estos bloques
declarativos y dependencias con coordenadas literales.

Un nombre corto busca el artefacto exacto
en Maven Central y pide elegir el grupo si hay varias coincidencias. Si omites
la versión, se consulta el metadato del artefacto y se guarda una versión estable
explícita. Para versiones con una convención distinta o repositorios distintos
de Maven Central, proporciona la coordenada completa.

La CLI evita duplicados, rechaza conflictos de versión explícitos y ejecuta
el wrapper del proyecto (`kotlin build` o `gradlew build`) una vez tras procesar todos los artefactos. Si el build falla,
conserva el cambio en el archivo de dependencias y devuelve el código de salida del wrapper.

## Repositorio y autoría

- Repositorio oficial en [GitHub](https://github.com/fgp555/kotlin-package-manager).
- Creado por [Frank GP](https://frankgp.com).
