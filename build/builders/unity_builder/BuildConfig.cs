// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

﻿
// IMPORTANT: Install this file in <ASSETS>/SGE/BuildUtils/Editor/BuildConfig.cs
using SGE.Tools;
using SGE.Utils;
using System;
using System.Collections.Generic;
using System.IO;
using System.Text;
using UnityEditor;
using UnityEditor.Build.Reporting;
using UnityEngine;

/// <summary>
/// Namespace dedicated to making build integrations with SG&E build system.
/// </summary>
namespace SGE.BuildUtils
{
  /// <summary>
  /// Class that holds the configuration information for the project for all profiles.
  /// This is the class that needs to be modified by the project maintainer.
  /// </summary>
  class BuildProfiles
  {
    /// <summary>
    ///  Returns the build target for a given profile.
    /// </summary>
    /// <returns>If the profile doesn't exist, should return BuildTarget.NoTarget.</returns>
    public BuildTarget GetBuildTarget(string profile)
    {
      throw new NotImplementedException();
    }

    /// <summary>
    ///  Returns the build options for a given profile.
    /// </summary>
    /// <returns>If the profile doesn't exist, should return false</returns>
    public bool TryGetBuildOptions(string profile,
                                   ref BuildPlayerOptions options)
    {
      throw new NotImplementedException();
    }
  }

  // -----------------------------------------------------------------------------------------------
  // BELOW THIS POINT IS PROJECT INDEPENDENT BUILD CODE, |BuildProcess| HOLDS THE SPECIFIC KNOW-HOW
  // FOR BUILDING THE UNITY PROJECT.
  // -----------------------------------------------------------------------------------------------

  /// <summary>
  /// Namespace dedicated to configuring builds for projects.
  /// </summary>
  public static class BuildConfig
  {
    public const string ActionSwitchTarget = "switch-target";
    public const string ActionBuild = "build";

    public static void Build()
    {
      string[] args = Environment.GetCommandLineArgs();
      string actionFlag = GetFlag(args, "--action");
      string outputFlag = GetFlag(args, "--output");
      string logsFlag = GetFlag(args, "--summary");
      string profileFlag = GetFlag(args, "--profile");

      ConsoleAndFileLog.logPath = logsFlag;

      BuildProfiles profiles = new BuildProfiles();
      switch (actionFlag)
      {
        case ActionSwitchTarget:
          {
            BuildTarget target = profiles.GetBuildTarget(profileFlag);
            if (target == BuildTarget.NoTarget)
            {
              ConsoleAndFileLog.LogError("Could not find target for " + profileFlag);
              EditorApplication.Exit(1);
              return;
            }
            var targetGroup = BuildPipeline.GetBuildTargetGroup(target);
            if (!EditorUserBuildSettings.SwitchActiveBuildTarget(targetGroup, target))
            {
              ConsoleAndFileLog.LogError("Could not switch target to " + target.ToString());
              EditorApplication.Exit(1);
              return;
            }
            return;
          }
        case ActionBuild:
          {
            BuildPlayerOptions options = new BuildPlayerOptions();
            if (!profiles.TryGetBuildOptions(profileFlag, ref options))
            {
              ConsoleAndFileLog.LogError("Could not find options for profile " + profileFlag);
              EditorApplication.Exit(1);
              return;
            }
            // Get the target and fix the output executable is needed.
            BuildTarget target = profiles.GetBuildTarget(profileFlag);
            if (target == BuildTarget.NoTarget)
            {
              ConsoleAndFileLog.LogError("Could not find target for " + profileFlag);
              EditorApplication.Exit(1);
              return;
            }
            if (target == BuildTarget.StandaloneWindows64)
            {
              if (!outputFlag.EndsWith(".exe"))
              {
                outputFlag += ".exe";
              }
            }
            options.locationPathName = outputFlag;
            if (!PerformBuild(target, options))
            {
              EditorApplication.Exit(1);
              return;
            }
            return;
          }
        default:
          {
            ConsoleAndFileLog.LogError("Invalid action: " + actionFlag);
            EditorApplication.Exit(1);
            return;
          }
      }
    }

    private static bool SetDefines(BuildTarget target, BuildTargetGroup targetGroup,
                                   string defines)
    {
      string currentDefines =
        PlayerSettings.GetScriptingDefineSymbolsForGroup(targetGroup);
      string newDefines = $"{currentDefines},{defines}";

      PlayerSettings.SetScriptingDefineSymbolsForGroup(targetGroup, newDefines);
      ConsoleAndFileLog.Log(
        $"SetDefines : setting {targetGroup} defines to '{newDefines}'. Result OK.");
      return true;
    }

    private static bool PerformBuild(BuildTarget target, BuildPlayerOptions options)
    {
      ConsoleAndFileLog.Log($" - Version={PlayerSettings.bundleVersion}\n");
      ConsoleAndFileLog.Log($" - Target={options.target}\n");
      ConsoleAndFileLog.Log($" - Scenes={string.Join(", ", options.scenes)}\n");
      ConsoleAndFileLog.Log($" - Options={options.options}\n");
      ConsoleAndFileLog.Log($" - OutputPath={options.locationPathName}\n");

      BuildReport report = BuildPipeline.BuildPlayer(options);
      BuildSummary summary = report.summary;
      if (summary.result == BuildResult.Succeeded)
      {
        ConsoleAndFileLog.Log("Build SUCCESS");
        ConsoleAndFileLog.Log("Total time: " + summary.totalTime.ToString());
        return true;
      }

      ConsoleAndFileLog.LogError("Build failed: " + summary.result.ToString());
      foreach (BuildStep step in report.steps)
      {
        StringBuilder builder = new StringBuilder();
        builder.AppendLine("Step: " + step.name);
        foreach (BuildStepMessage message in step.messages)
        {
          builder.AppendLine(message.type.ToString() + " - " + message.content);
        }

        ConsoleAndFileLog.LogError(builder.ToString());
      }
      return false;
    }

    private static string GetFlag(string[] args, string flagName)
    {
      for (int i = 0; i < args.Length; i++)
      {
        if (args[i] == flagName && i < args.Length - 1)
        {
          return args[i + 1];
        }
      }
      throw new ArgumentException("Flag " + flagName + " not found");
    }
  }

  /// <summary>
  /// Class that writes to both Unity Logs and a given log file.
  /// </summary>
  class ConsoleAndFileLog
  {
    public static string logPath;

    public static void Log(string log)
    {
      Debug.Log(log);
      File.AppendAllText(logPath, log + Environment.NewLine);
    }

    public static void LogError(string log)
    {
      Log("[ERROR] " + log);
    }
  }
}
