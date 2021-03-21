# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

from collections import defaultdict
import os

# 16384

PLANETS = ['ordmantell', 'abafar', 'aeosprime', 'agamar', 'ahchto', 'ajankloss', 'akiva', 'alderaan', 'alzociii', 'ando', 'anoat', 'arvala', 'atollon', 'batuu', 'behpour', 'bespin', 'bogano', 'bracca', 'cantonica', 'castilon', 'catoneimoidia', 'celsor', 'chandrila', 'christophsis', 'concorddawn', 'corellia', 'coruscant', 'crait', 'dqar', 'dagobah', 'dandoran', 'dantooine', 'dathomir', 'devaron', 'eadu', 'endor', 'eriadu', 'exegol', 'felucia', 'florrum', 'fondor', 'geonosis', 'hosnianprime', 'hoth', 'iego', 'ilum', 'iridonia', 'jakku', 'jedha',
           'kamino', 'kashyyyk', 'kefbir', 'kessel', 'kijimi', 'kuat', 'lahmu', 'lirasan', 'lothal', 'lothominor', 'malachor', 'malastare', 'mandalore', 'maridun', 'mimban', 'moncala', 'moraband', 'mortis', 'mustafar', 'mygeeto', 'naboo', 'nalhutta', 'nevarro', 'nur', 'onderon', 'pasaana', 'pillio', 'polismassa', 'rishi', 'rodia', 'ruusan', 'ryloth', 'saleucami', 'savareen', 'scarif', 'serenno', 'shili', 'sorgan', 'starkillerbase', 'subterrel', 'sullust', 'takodana', 'tatooine', 'toydaria', 'trandosha', 'umbara', 'utapau', 'vandor', 'vardos', 'wobani', 'yavin', 'zeffo', 'zygerria']

planet_index = 0
planet_systems = defaultdict(int)

for i in range(10000):
  planet = PLANETS[planet_index]
  planet_index += 1
  if planet_index >= len(PLANETS):
    planet_index = 0

  planet_systems[planet] += 1
  
  filename = '%s-%d.dat' % (planet, planet_systems[planet])
  print(filename)
  os.system('head -c 16384 </dev/urandom > %s' % filename)
  